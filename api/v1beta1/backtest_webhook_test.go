package v1beta1

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func newTestBacktest() *Backtest {
	return &Backtest{
		Spec: BacktestSpec{
			RunSpec: RunSpec{
				ConfigRef:   corev1.LocalObjectReference{Name: "my-config"},
				StrategyRef: corev1.LocalObjectReference{Name: "my-strategy"},
			},
		},
	}
}

func TestValidateBacktestSpec(t *testing.T) {
	tests := []struct {
		name        string
		backtest    *Backtest
		wantErr     bool
		errContains string
	}{
		{name: "valid", backtest: newTestBacktest(), wantErr: false},
		{
			name: "missing configRef.name",
			backtest: func() *Backtest {
				b := newTestBacktest()
				b.Spec.ConfigRef.Name = ""
				return b
			}(),
			wantErr:     true,
			errContains: "spec.configRef.name is required",
		},
		{
			name: "whitespace-only configRef.name",
			backtest: func() *Backtest {
				b := newTestBacktest()
				b.Spec.ConfigRef.Name = "   "
				return b
			}(),
			wantErr:     true,
			errContains: "spec.configRef.name is required",
		},
		{
			name: "missing strategyRef.name",
			backtest: func() *Backtest {
				b := newTestBacktest()
				b.Spec.StrategyRef.Name = ""
				return b
			}(),
			wantErr:     true,
			errContains: "spec.strategyRef.name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBacktestSpec(tt.backtest)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBacktestSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestValidateExtraArgs(t *testing.T) {
	withAnnotation := func(b *Backtest) *Backtest {
		b.Annotations = map[string]string{allowExtraArgsAnnotation: "true"}
		return b
	}

	tests := []struct {
		name        string
		backtest    *Backtest
		wantErr     bool
		errContains string
	}{
		{name: "no extraArgs at all", backtest: newTestBacktest(), wantErr: false},
		{
			name: "extraArgs without the annotation is rejected",
			backtest: func() *Backtest {
				b := newTestBacktest()
				b.Spec.ExtraArgs = []string{"--enable-position-stacking"}
				return b
			}(),
			wantErr:     true,
			errContains: "freqtrade.io/allow-extra-args",
		},
		{
			name: "extraArgs with the annotation set to something other than true is rejected",
			backtest: func() *Backtest {
				b := newTestBacktest()
				b.Spec.ExtraArgs = []string{"--enable-position-stacking"}
				b.Annotations = map[string]string{allowExtraArgsAnnotation: "yes"}
				return b
			}(),
			wantErr: true,
		},
		{
			name: "extraArgs with the annotation is accepted",
			backtest: func() *Backtest {
				b := withAnnotation(newTestBacktest())
				b.Spec.ExtraArgs = []string{"--enable-position-stacking"}
				return b
			}(),
			wantErr: false,
		},
		{
			name: "denylisted flag is rejected even with the annotation",
			backtest: func() *Backtest {
				b := withAnnotation(newTestBacktest())
				b.Spec.ExtraArgs = []string{"--db-url", "sqlite:///something-else.sqlite"}
				return b
			}(),
			wantErr:     true,
			errContains: `"--db-url"`,
		},
		{
			name: "denylisted flag using = form is rejected",
			backtest: func() *Backtest {
				b := withAnnotation(newTestBacktest())
				b.Spec.ExtraArgs = []string{"--config=/tmp/evil.json"}
				return b
			}(),
			wantErr:     true,
			errContains: `"--config"`,
		},
		{
			name: "shell metacharacter is rejected even with the annotation",
			backtest: func() *Backtest {
				b := withAnnotation(newTestBacktest())
				b.Spec.ExtraArgs = []string{"--timerange 20230101-; rm -rf /"}
				return b
			}(),
			wantErr:     true,
			errContains: "shell metacharacter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExtraArgs(tt.backtest)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateExtraArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

func TestBacktestCustomValidator_ValidateCreateAndUpdate(t *testing.T) {
	v := &BacktestCustomValidator{}
	ctx := context.Background()
	valid := newTestBacktest()

	if _, err := v.ValidateCreate(ctx, valid); err != nil {
		t.Errorf("ValidateCreate: unexpected error for a valid Backtest: %v", err)
	}
	if _, err := v.ValidateUpdate(ctx, valid, valid); err != nil {
		t.Errorf("ValidateUpdate: unexpected error for a valid Backtest: %v", err)
	}
	if _, err := v.ValidateDelete(ctx, valid); err != nil {
		t.Errorf("ValidateDelete: deletion must never be rejected, got %v", err)
	}

	invalid := newTestBacktest()
	invalid.Spec.ConfigRef.Name = ""
	if _, err := v.ValidateCreate(ctx, invalid); err == nil {
		t.Error("ValidateCreate: expected an error for a missing configRef.name")
	}

	if _, err := v.ValidateCreate(ctx, &corev1.Pod{}); err == nil {
		t.Error("ValidateCreate: expected an error for the wrong object type")
	}
}
