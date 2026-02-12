package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"

	"github.com/msimon/nauts/jwt"
)

const (
	// AuthCalloutSubject is the NATS subject for auth callout requests.
	AuthCalloutSubject = "$SYS.REQ.USER.AUTH"

	// ServerXKeyHeader is the header containing the server's xkey public key.
	ServerXKeyHeader = "Nats-Server-Xkey"
)

// CalloutConfig holds configuration for the auth callout service.
type CalloutConfig struct {
	// NatsURL is the NATS server URL.
	NatsURL string

	// NatsCredentials is the path to the credentials file for connecting to NATS.
	// Mutually exclusive with NatsNkey.
	NatsCredentials string

	// NatsNkey is the path to the nkey seed file for NATS authentication.
	// Mutually exclusive with NatsCredentials.
	NatsNkey string

	// XKeySeed is the service's curve key seed for encryption/decryption.
	// Required for encrypted auth callout.
	XKeySeed string

	// DefaultTTL is the default JWT time-to-live.
	DefaultTTL time.Duration
}

// CalloutService handles NATS auth callout requests.
type CalloutService struct {
	controller *AuthController
	config     CalloutConfig

	curveKeyPair nkeys.KeyPair
	nc           *nats.Conn
	sub          *nats.Subscription
	logger       Logger

	done   chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// CalloutOption configures a CalloutService.
type CalloutOption func(*CalloutService)

// WithCalloutLogger sets a custom logger for the callout service.
func WithCalloutLogger(l Logger) CalloutOption {
	return func(s *CalloutService) {
		s.logger = l
	}
}

// NewCalloutService creates a new CalloutService.
func NewCalloutService(controller *AuthController, config CalloutConfig, opts ...CalloutOption) (*CalloutService, error) {
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

	s := &CalloutService{
		controller: controller,
		config:     config,
		logger:     &defaultLogger{},
		done:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Parse xkey seed if provided
	if config.XKeySeed != "" {
		kp, err := nkeys.FromSeed([]byte(config.XKeySeed))
		if err != nil {
			return nil, fmt.Errorf("parsing xkey seed: %w", err)
		}
		s.curveKeyPair = kp
	}

	return s, nil
}

// Start connects to NATS and begins handling auth callout requests.
// This method blocks until Stop is called or the context is cancelled.
func (s *CalloutService) Start(ctx context.Context) error {
	// Build NATS connection options
	opts := []nats.Option{
		nats.Name("nauts-auth-callout"),
	}

	// Add authentication option
	if s.config.NatsCredentials != "" {
		opts = append(opts, nats.UserCredentials(s.config.NatsCredentials))
	} else if s.config.NatsNkey != "" {
		opt, err := nats.NkeyOptionFromSeed(s.config.NatsNkey)
		if err != nil {
			return fmt.Errorf("loading nkey from %s: %w", s.config.NatsNkey, err)
		}
		opts = append(opts, opt)
	}

	// Connect to NATS
	nc, err := nats.Connect(s.config.NatsURL, opts...)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	s.nc = nc

	// Subscribe to auth callout subject
	sub, err := nc.Subscribe(AuthCalloutSubject, s.handleRequest)
	if err != nil {
		nc.Close()
		return fmt.Errorf("subscribing to %s: %w", AuthCalloutSubject, err)
	}
	s.sub = sub

	s.logger.Info("auth callout service started, listening on %s", AuthCalloutSubject)

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		s.logger.Info("context cancelled, shutting down")
	case <-s.done:
		s.logger.Info("stop requested, shutting down")
	}

	return s.shutdown()
}

// Stop signals the service to shut down gracefully.
func (s *CalloutService) Stop() error {
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
func (s *CalloutService) shutdown() error {
	// Drain subscription to stop receiving new requests
	if s.sub != nil {
		if err := s.sub.Drain(); err != nil {
			s.logger.Warn("error draining subscription: %v", err)
		}
	}

	// Wait for in-flight requests to complete
	s.wg.Wait()

	// Close NATS connection
	if s.nc != nil {
		s.nc.Close()
	}

	s.logger.Info("auth callout service stopped")
	return nil
}

type ResponseConfig struct {
	UserNkey   string
	ServerId   string
	ServerXkey string
}

// handleRequest processes an auth callout request.
func (s *CalloutService) handleRequest(msg *nats.Msg) {
	s.wg.Add(1)
	defer s.wg.Done()

	ctx := context.Background()

	// setup response config
	responseConfig := ResponseConfig{
		UserNkey:   "",
		ServerId:   "",
		ServerXkey: "",
	}

	// Extract server xkey from headers
	serverXKey := ""
	if msg.Header != nil {
		serverXKey = msg.Header.Get(ServerXKeyHeader)
	}
	responseConfig.ServerXkey = serverXKey

	// Decrypt request if we have an xkey
	requestData := msg.Data
	if s.curveKeyPair != nil && serverXKey != "" {
		decrypted, err := s.curveKeyPair.Open(msg.Data, serverXKey)
		if err != nil {
			s.logger.Warn("failed to decrypt request: %v", err)
			s.respondWithError(msg, responseConfig, "authentication failed")
			return
		}
		requestData = decrypted
	}

	// Decode auth request claims
	authReq, err := natsjwt.DecodeAuthorizationRequestClaims(string(requestData))
	if err != nil {
		s.logger.Warn("failed to decode auth request: %v", err)
		s.respondWithError(msg, responseConfig, "authentication failed")
		return
	}
	responseConfig.UserNkey = authReq.UserNkey
	responseConfig.ServerId = authReq.Server.ID

	s.logger.Debug("auth request received")

	// Authenticate
	result, err := s.controller.Authenticate(ctx, authReq.ConnectOptions, authReq.UserNkey, s.config.DefaultTTL)
	if err != nil {
		s.logger.Warn("authentication failed: %v", err)
		s.respondWithError(msg, responseConfig, "authentication failed")
		return
	}
	// update user public key in response config
	responseConfig.UserNkey = result.UserPublicKey

	// Get account for IssuerAccount
	account, err := s.controller.AccountProvider().GetAccount(ctx, result.User.Account)
	if err != nil {
		s.logger.Warn("failed to get account for user %s: %v", result.User.ID, err)
		s.respondWithError(msg, responseConfig, "internal error")
		return
	}

	// In operator mode, use signing key's public key for IssuerAccount
	// In non-operator mode, use account's public key (though IssuerAccount is not set)
	issuerAccount := account.PublicKey()
	if s.controller.AccountProvider().IsOperatorMode() {
		issuerAccount = account.Signer().PublicKey()
	}

	// Build auth response
	s.respondWithSuccess(msg, responseConfig, result.JWT, issuerAccount)
}

// respondWithError sends an error response.
func (s *CalloutService) respondWithError(msg *nats.Msg, responseConfig ResponseConfig, errMsg string) {
	resp := natsjwt.NewAuthorizationResponseClaims(responseConfig.UserNkey)
	resp.Audience = responseConfig.ServerId
	resp.Error = errMsg
	s.sendResponse(msg, responseConfig.ServerXkey, resp)
}

// respondWithSuccess sends a success response with the user JWT.
// In operator mode, IssuerAccount is set to the signing key's public key.
// In non-operator mode, IssuerAccount is NOT set because the NATS server
// derives the target account from the user JWT's Audience field instead.
func (s *CalloutService) respondWithSuccess(msg *nats.Msg, responseConfig ResponseConfig, userJWT, issuerAccount string) {
	resp := natsjwt.NewAuthorizationResponseClaims(responseConfig.UserNkey)
	resp.Jwt = userJWT
	resp.Audience = responseConfig.ServerId

	// In operator mode, set IssuerAccount to the signing key's public key
	if s.controller.AccountProvider().IsOperatorMode() {
		resp.IssuerAccount = issuerAccount
	}

	s.sendResponse(msg, responseConfig.ServerXkey, resp)
}

// sendResponse encodes, optionally encrypts, and sends the response.
func (s *CalloutService) sendResponse(msg *nats.Msg, serverXKey string, resp *natsjwt.AuthorizationResponseClaims) {
	// Get the account signer for encoding the response
	// The auth callout response must be signed by the account that's configured as the auth issuer
	// For simplicity, we use the first available account's signer
	ctx := context.Background()
	account, err := s.controller.AccountProvider().GetAccount(ctx, "AUTH")
	if err != nil {
		s.logger.Warn("failed to get account for response signing: %v", err)
		return
	}

	// Encode the response (signed by account)
	token, err := resp.Encode(jwt.NewSignerAdapter(account.Signer()))
	if err != nil {
		s.logger.Warn("failed to encode response: %v", err)
		return
	}

	responseData := []byte(token)

	// Encrypt response if we have xkey and server provided its key
	if s.curveKeyPair != nil && serverXKey != "" {
		encrypted, err := s.curveKeyPair.Seal(responseData, serverXKey)
		if err != nil {
			s.logger.Warn("failed to encrypt response: %v", err)
			return
		}
		responseData = encrypted
	}

	// Send response
	if err := msg.Respond(responseData); err != nil {
		s.logger.Warn("failed to send response: %v", err)
	}
}
