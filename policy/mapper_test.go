package policy

import (
	"testing"
)

func TestMapActionToPermissions_NATS(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		nrnStr string
		want   []Permission
	}{
		{
			name:   "nats.pub simple",
			action: ActionNATSPub,
			nrnStr: "nats:orders",
			want: []Permission{
				{Type: PermPub, Subject: "orders"},
			},
		},
		{
			name:   "nats.pub wildcard",
			action: ActionNATSPub,
			nrnStr: "nats:orders.>",
			want: []Permission{
				{Type: PermPub, Subject: "orders.>"},
			},
		},
		{
			name:   "nats.sub simple",
			action: ActionNATSSub,
			nrnStr: "nats:orders",
			want: []Permission{
				{Type: PermSub, Subject: "orders"},
			},
		},
		{
			name:   "nats.sub with queue",
			action: ActionNATSSub,
			nrnStr: "nats:orders:workers",
			want: []Permission{
				{Type: PermSub, Subject: "orders", Queue: "workers"},
			},
		},
		{
			name:   "nats.req",
			action: ActionNATSReq,
			nrnStr: "nats:orders.request",
			want: []Permission{
				{Type: PermPub, Subject: "orders.request"},
				{Type: PermSub, Subject: "_INBOX.>"},
			},
		},
		{
			name:   "nats.pub wrong type",
			action: ActionNATSPub,
			nrnStr: "js:ORDERS",
			want:   []Permission{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseResource(tt.nrnStr)
			if err != nil {
				t.Fatalf("Failed to parse Resource: %v", err)
			}

			got := MapActionToPermissions(tt.action, n)

			if len(got) != len(tt.want) {
				t.Errorf("MapActionToPermissions() got %d permissions, want %d", len(got), len(tt.want))
				return
			}
			for i, w := range tt.want {
				if got[i].Type != w.Type || got[i].Subject != w.Subject || got[i].Queue != w.Queue {
					t.Errorf("MapActionToPermissions()[%d] = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestMapActionToPermissions_JS(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		nrnStr string
		want   []Permission
	}{
		{
			name:   "js.readStream specific",
			action: ActionJSReadStream,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.ORDERS"},
			},
		},
		{
			name:   "js.readStream wildcard",
			action: ActionJSReadStream,
			nrnStr: "js:*",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.*"},
				{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
				{Type: PermPub, Subject: "$JS.API.STREAM.NAMES"},
			},
		},
		{
			name:   "js.writeStream",
			action: ActionJSWriteStream,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.CREATE.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.STREAM.UPDATE.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.STREAM.PURGE.ORDERS"},
			},
		},
		{
			name:   "js.deleteStream",
			action: ActionJSDeleteStream,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.DELETE.ORDERS"},
			},
		},
		{
			name:   "js.readConsumer specific",
			action: ActionJSReadConsumer,
			nrnStr: "js:ORDERS:processor",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO.ORDERS.processor"},
			},
		},
		{
			name:   "js.readConsumer wildcard",
			action: ActionJSReadConsumer,
			nrnStr: "js:ORDERS:*",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.LIST.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.NAMES.ORDERS"},
			},
		},
		{
			name:   "js.readConsumer stream only",
			action: ActionJSReadConsumer,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.LIST.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.NAMES.ORDERS"},
			},
		},
		{
			name:   "js.writeConsumer",
			action: ActionJSWriteConsumer,
			nrnStr: "js:ORDERS:processor",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.ORDERS.processor.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.processor"},
			},
		},
		{
			name:   "js.deleteConsumer",
			action: ActionJSDeleteConsumer,
			nrnStr: "js:ORDERS:processor",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DELETE.ORDERS.processor"},
			},
		},
		{
			name:   "js.consume",
			action: ActionJSConsume,
			nrnStr: "js:ORDERS:processor",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.ORDERS.processor"},
				{Type: PermPub, Subject: "$JS.ACK.ORDERS.processor.>"},
			},
		},
		{
			name:   "js.readStream wrong type",
			action: ActionJSReadStream,
			nrnStr: "nats:orders",
			want:   []Permission{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseResource(tt.nrnStr)
			if err != nil {
				t.Fatalf("Failed to parse Resource: %v", err)
			}

			got := MapActionToPermissions(tt.action, n)

			if len(got) != len(tt.want) {
				t.Errorf("MapActionToPermissions() got %d permissions, want %d", len(got), len(tt.want))
				for i, p := range got {
					t.Logf("  got[%d]: %+v", i, p)
				}
				for i, p := range tt.want {
					t.Logf("  want[%d]: %+v", i, p)
				}
				return
			}
			for i, w := range tt.want {
				if got[i].Type != w.Type || got[i].Subject != w.Subject || got[i].Queue != w.Queue {
					t.Errorf("MapActionToPermissions()[%d] = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestMapActionToPermissions_KV(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		nrnStr string
		want   []Permission
	}{
		{
			name:   "kv.read specific key",
			action: ActionKVRead,
			nrnStr: "kv:config:app.settings",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.app.settings"},
			},
		},
		{
			name:   "kv.read bucket only",
			action: ActionKVRead,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.>"},
			},
		},
		{
			name:   "kv.write specific key",
			action: ActionKVWrite,
			nrnStr: "kv:config:app.settings",
			want: []Permission{
				{Type: PermPub, Subject: "$KV.config.app.settings"},
			},
		},
		{
			name:   "kv.write bucket only",
			action: ActionKVWrite,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$KV.config.>"},
			},
		},
		{
			name:   "kv.watchBucket",
			action: ActionKVWatchBucket,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config.*.$KV.config.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DELETE.KV_config.>"},
				{Type: PermSub, Subject: "_INBOX.>"},
				{Type: PermPub, Subject: "$JS.FC.KV_config.>"},
			},
		},
		{
			name:   "kv.readBucket",
			action: ActionKVReadBucket,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
			},
		},
		{
			name:   "kv.writeBucket",
			action: ActionKVWriteBucket,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.CREATE.KV_config"},
			},
		},
		{
			name:   "kv.deleteBucket",
			action: ActionKVDeleteBucket,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.DELETE.KV_config"},
			},
		},
		{
			name:   "kv.read wrong type",
			action: ActionKVRead,
			nrnStr: "nats:orders",
			want:   []Permission{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseResource(tt.nrnStr)
			if err != nil {
				t.Fatalf("Failed to parse Resource: %v", err)
			}

			got := MapActionToPermissions(tt.action, n)

			if len(got) != len(tt.want) {
				t.Errorf("MapActionToPermissions() got %d permissions, want %d", len(got), len(tt.want))
				for i, p := range got {
					t.Logf("  got[%d]: %+v", i, p)
				}
				for i, p := range tt.want {
					t.Logf("  want[%d]: %+v", i, p)
				}
				return
			}
			for i, w := range tt.want {
				if got[i].Type != w.Type || got[i].Subject != w.Subject || got[i].Queue != w.Queue {
					t.Errorf("MapActionToPermissions()[%d] = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestMapActionToPermissions_UnknownAction(t *testing.T) {
	n, _ := ParseResource("nats:orders")
	got := MapActionToPermissions(Action("unknown"), n)
	if len(got) != 0 {
		t.Errorf("MapActionToPermissions() expected empty permissions for unknown action, got %d", len(got))
	}
}
