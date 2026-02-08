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
			name:   "nats.service",
			action: ActionNATSService,
			nrnStr: "nats:orders.request",
			want: []Permission{
				{Type: PermSub, Subject: "orders.request"},
				{Type: PermResp},
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
			name:   "js.manage specific",
			action: ActionJSManage,
			nrnStr: "js:ORDERS",
			want: []Permission{
				// js.consume permissions
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.ACK.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.FC.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS.>"},
				// js.manage additional permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.*.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.STREAM.MSG.*.ORDERS"},
			},
		},
		{
			name:   "js.manage wildcard",
			action: ActionJSManage,
			nrnStr: "js:*",
			want: []Permission{
				// js.consume permissions
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.*"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.*.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.*.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.*.*"},
				{Type: PermPub, Subject: "$JS.ACK.*.>"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE.*.*"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK.*.*"},
				{Type: PermPub, Subject: "$JS.FC.*.>"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.*"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.*.>"},
				// js.manage additional permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.*.*"},
				{Type: PermPub, Subject: "$JS.API.STREAM.MSG.*.*"},
				{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
				{Type: PermPub, Subject: "$JS.API.STREAM.NAMES"},
			},
		},
		{
			name:   "js.view specific",
			action: ActionJSView,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.LIST.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.NAMES.ORDERS"},
			},
		},
		{
			name:   "js.consume specific",
			action: ActionJSConsume,
			nrnStr: "js:ORDERS:processor",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO.ORDERS.processor"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.processor"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.ORDERS.processor"},
				{Type: PermPub, Subject: "$JS.ACK.ORDERS.processor.>"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.FC.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS.>"},
			},
		},
		{
			name:   "js.consume wildcard consumers",
			action: ActionJSConsume,
			nrnStr: "js:ORDERS:*",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.ACK.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.FC.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS.>"},
			},
		},
		{
			name:   "js.consume all consumers",
			action: ActionJSConsume,
			nrnStr: "js:ORDERS",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.*.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.ACK.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK.ORDERS.*"},
				{Type: PermPub, Subject: "$JS.FC.ORDERS.>"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.ORDERS.>"},
			},
		},
		{
			name:   "js.view wrong type",
			action: ActionJSView,
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
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.app.settings"},
				{Type: PermSub, Subject: "$KV.config.app.settings"},
			},
		},
		{
			name:   "kv.read bucket only",
			action: ActionKVRead,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config.>"},
				{Type: PermPub, Subject: "$JS.FC.KV_config.>"},
				{Type: PermSub, Subject: "$KV.config.>"},
			},
		},
		{
			name:   "kv.edit specific key",
			action: ActionKVEdit,
			nrnStr: "kv:config:app.settings",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.app.settings"},
				{Type: PermSub, Subject: "$KV.config.app.settings"},
				{Type: PermPub, Subject: "$KV.config.app.settings"},
			},
		},
		{
			name:   "kv.edit bucket only",
			action: ActionKVEdit,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config.>"},
				{Type: PermPub, Subject: "$JS.FC.KV_config.>"},
				{Type: PermSub, Subject: "$KV.config.>"},
				{Type: PermPub, Subject: "$KV.config.>"},
			},
		},
		{
			name:   "kv.view specific bucket",
			action: ActionKVView,
			nrnStr: "kv:config",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
			},
		},
		{
			name:   "kv.view wildcard bucket",
			action: ActionKVView,
			nrnStr: "kv:*",
			want: []Permission{
				{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.*"},
			},
		},
		{
			name:   "kv.manage specific bucket",
			action: ActionKVManage,
			nrnStr: "kv:config",
			want: []Permission{
				// kv.read permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_config"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_config.$KV.config.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_config.>"},
				{Type: PermPub, Subject: "$JS.FC.KV_config.>"},
				{Type: PermSub, Subject: "$KV.config.>"},
				// kv.manage additional permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.*.KV_config"},
			},
		},
		{
			name:   "kv.manage wildcard bucket",
			action: ActionKVManage,
			nrnStr: "kv:*",
			want: []Permission{
				// kv.read permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_*"},
				{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_*.$KV.*.>"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_*"},
				{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_*.>"},
				{Type: PermPub, Subject: "$JS.FC.KV_*.>"},
				{Type: PermSub, Subject: "$KV.*.>"},
				// kv.manage additional permissions
				{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
				{Type: PermPub, Subject: "$JS.API.STREAM.INFO.*"},
				{Type: PermPub, Subject: "$JS.API.STREAM.*.*"},
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
