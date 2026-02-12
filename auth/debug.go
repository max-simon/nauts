package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/msimon/nauts/identity"
)

const (
	// DebugSubject is the NATS subject for auth debug requests.
	DebugSubject = "nauts.debug"
)

// DebugConfig holds configuration for the auth debug service.
type DebugConfig struct {
	// NatsURL is the NATS server URL.
	NatsURL string

	// NatsCredentials is the path to the credentials file for connecting to NATS.
	// Mutually exclusive with NatsNkey.
	NatsCredentials string

	// NatsNkey is the path to the nkey seed file for NATS authentication.
	// Mutually exclusive with NatsCredentials.
	NatsNkey string

	// DefaultTTL is the default JWT time-to-live.
	DefaultTTL time.Duration
}

// DebugService handles NATS debug requests.
type DebugService struct {
	controller *AuthController
	config     DebugConfig

	nc     *nats.Conn
	sub    *nats.Subscription
	logger Logger

	done   chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// DebugOption configures a DebugService.
type DebugOption func(*DebugService)

// WithDebugLogger sets a custom logger for the debug service.
func WithDebugLogger(l Logger) DebugOption {
	return func(s *DebugService) {
		s.logger = l
	}
}

// NewDebugService creates a new DebugService.
func NewDebugService(controller *AuthController, config DebugConfig, opts ...DebugOption) (*DebugService, error) {
	if controller == nil {
		return nil, errors.New("controller is required")
	}
	// Validate authentication options: either credentials file or nkey
	hasCredentials := config.NatsCredentials != ""
	hasNkey := config.NatsNkey != ""
	if !hasCredentials && !hasNkey {
		return nil, errors.New("NATS authentication required: set NatsCredentials or NatsNkey")
	}
	if hasCredentials && hasNkey {
		return nil, errors.New("NatsCredentials and NatsNkey are mutually exclusive")
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = time.Hour
	}
	if config.NatsURL == "" {
		config.NatsURL = nats.DefaultURL
	}
	if os.Getenv("NATS_URL") != "" {
		config.NatsURL = os.Getenv("NATS_URL")
	}

	s := &DebugService{
		controller: controller,
		config:     config,
		logger:     &defaultLogger{},
		done:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Start connects to NATS and begins handling debug requests.
// This method blocks until Stop is called or the context is cancelled.
func (s *DebugService) Start(ctx context.Context) error {
	opts := []nats.Option{
		nats.Name("nauts-auth-debug"),
	}

	if s.config.NatsCredentials != "" {
		opts = append(opts, nats.UserCredentials(s.config.NatsCredentials))
	} else if s.config.NatsNkey != "" {
		opt, err := nats.NkeyOptionFromSeed(s.config.NatsNkey)
		if err != nil {
			return fmt.Errorf("loading nkey from %s: %w", s.config.NatsNkey, err)
		}
		opts = append(opts, opt)
	}

	nc, err := nats.Connect(s.config.NatsURL, opts...)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	s.nc = nc

	sub, err := nc.Subscribe(DebugSubject, s.handleRequest)
	if err != nil {
		nc.Close()
		return fmt.Errorf("subscribing to %s: %w", DebugSubject, err)
	}
	s.sub = sub

	s.logger.Info("auth debug service started, listening on %s", DebugSubject)

	select {
	case <-ctx.Done():
		s.logger.Info("context cancelled, shutting down")
	case <-s.done:
		s.logger.Info("stop requested, shutting down")
	}

	return s.shutdown()
}

// Stop signals the service to shut down gracefully.
func (s *DebugService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return nil
}

// shutdown performs graceful shutdown.
func (s *DebugService) shutdown() error {
	if s.sub != nil {
		if err := s.sub.Drain(); err != nil {
			s.logger.Warn("error draining subscription: %v", err)
		}
	}

	s.wg.Wait()

	if s.nc != nil {
		s.nc.Close()
	}

	s.logger.Info("auth debug service stopped")
	return nil
}

type debugError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type debugRequest struct {
	User    *identity.User `json:"user"`
	Account string         `json:"account"`
}

type debugResponse struct {
	Request           *debugRequest           `json:"request"`
	CompilationResult *NautsCompilationResult `json:"compilation_result"`
	Error             *debugError             `json:"error"`
}

func (r *debugResponse) setError(code, message string) {
	if r.Error.Code != "" {
		return
	}
	r.Error.Code = code
	r.Error.Message = message
}

// handleRequest processes a debug request.
func (s *DebugService) handleRequest(msg *nats.Msg) {
	s.wg.Add(1)
	defer s.wg.Done()

	ctx := context.Background()
	resp := debugResponse{}

	// get debugRequest from msg.Data json
	var req debugRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		resp.setError("invalid_request", "failed to parse debug request")
		s.respondWithJSON(msg, resp)
		return
	}
	resp.Request = &req

	// scope user
	scopedUser, err := s.controller.ScopeUserToAccount(ctx, req.User, req.Account)
	if err != nil {
		resp.setError("compile_error", fmt.Sprintf("failed to scope user %s to account %s", req.User.ID, req.Account))
		s.respondWithJSON(msg, resp)
		return
	}

	// compile permissions
	compileResult, err := s.controller.CompileNatsPermissions(ctx, scopedUser)
	if err != nil {
		resp.setError("compile_error", fmt.Sprintf("failed to compile permissions for user %s", scopedUser.ID))
		s.respondWithJSON(msg, resp)
		return
	}
	resp.CompilationResult = compileResult

	s.respondWithJSON(msg, resp)
}

func (s *DebugService) respondWithJSON(msg *nats.Msg, resp debugResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Warn("failed to encode debug response: %v", err)
		return
	}
	if err := msg.Respond(data); err != nil {
		s.logger.Warn("failed to send debug response: %v", err)
	}
}
