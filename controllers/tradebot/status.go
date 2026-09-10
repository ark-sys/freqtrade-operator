package tradebot

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// patchTradeBotStatus wraps shared.PatchStatus so every TradeBot status
// write this package makes - the poller's own observations and every
// reconcile's own status patch alike - goes through v1beta1 instead of
// tradeBot's own v1alpha1 type (P4-4).
//
// Why: v1alpha1 has no Spec.State field at all, and this is not just a
// theoretical gap - verified directly against envtest: a v1alpha1-typed
// Status().Update() round-trips the *whole* object through the same
// conversion a full Update() does, to reach the storage version, and
// silently resets Spec.State to its CRD default on every single status
// write, not just a spec write. Since a status patch happens on every
// reconcile (and every poll), leaving any of these nine call sites on
// v1alpha1 would mean a Stopped bot gets silently reset back to Running
// on the very next unrelated status update - regardless of the
// finalizer-add/remove fix in main.go's Reconcile, which only closed the
// spec-write half of this.
//
// mutate is untouched by this: it still reads/writes tradeBot.Status
// exactly as every caller already does. Only the type actually
// persisted changes - tradeBot.Status is kept in sync with what got
// written, both before mutate runs (so it sees the freshest stored
// status, matching shared.PatchStatus's own re-Get-on-each-retry
// contract) and after (so it's implicitly building on the copy about to
// be persisted).
func patchTradeBotStatus(
	ctx context.Context, c client.Client, tradeBot *freqtradev1alpha1.TradeBot, mutate func(),
) error {
	beta := &freqtradev1beta1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: tradeBot.Name, Namespace: tradeBot.Namespace}}
	return shared.PatchStatus(ctx, c, beta, func() {
		copyTradeBotStatusFromBeta(tradeBot, beta)
		mutate()
		copyTradeBotStatusToBeta(beta, tradeBot)
	})
}

// copyTradeBotStatusFromBeta and copyTradeBotStatusToBeta copy
// TradeBotStatus between the two API versions - field-for-field
// identical (see api/v1beta1/tradebot_types.go's own doc comment on
// TBAppConfig/BotStatus/etc.), so this is a pure pass-through in either
// direction, not a lossy conversion. Conditions is the same
// metav1.Condition slice type in both packages and is assigned
// directly; BotStatus is a distinct named type per package (same
// reason TBAppConfig etc. are redeclared, not aliased - see that same
// doc comment), so its handful of fields are copied explicitly instead.
func copyTradeBotStatusFromBeta(dst *freqtradev1alpha1.TradeBot, src *freqtradev1beta1.TradeBot) {
	dst.Status.Phase = src.Status.Phase
	dst.Status.Message = src.Status.Message
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.ObservedGeneration = src.Status.ObservedGeneration
	dst.Status.AppliedConfigHash = src.Status.AppliedConfigHash
	dst.Status.ResolvedImage = src.Status.ResolvedImage
	dst.Status.Bot = nil
	if src.Status.Bot != nil {
		bot := freqtradev1alpha1.BotStatus{
			State: src.Status.Bot.State, Version: src.Status.Bot.Version, DryRun: src.Status.Bot.DryRun,
			OpenTrades: src.Status.Bot.OpenTrades, MaxOpenTrades: src.Status.Bot.MaxOpenTrades,
			TotalProfitAbs: src.Status.Bot.TotalProfitAbs, TotalProfitPct: src.Status.Bot.TotalProfitPct,
			LastPollTime: src.Status.Bot.LastPollTime, LastPollError: src.Status.Bot.LastPollError,
		}
		dst.Status.Bot = &bot
	}
}

func copyTradeBotStatusToBeta(dst *freqtradev1beta1.TradeBot, src *freqtradev1alpha1.TradeBot) {
	dst.Status.Phase = src.Status.Phase
	dst.Status.Message = src.Status.Message
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.ObservedGeneration = src.Status.ObservedGeneration
	dst.Status.AppliedConfigHash = src.Status.AppliedConfigHash
	dst.Status.ResolvedImage = src.Status.ResolvedImage
	dst.Status.Bot = nil
	if src.Status.Bot != nil {
		bot := freqtradev1beta1.BotStatus{
			State: src.Status.Bot.State, Version: src.Status.Bot.Version, DryRun: src.Status.Bot.DryRun,
			OpenTrades: src.Status.Bot.OpenTrades, MaxOpenTrades: src.Status.Bot.MaxOpenTrades,
			TotalProfitAbs: src.Status.Bot.TotalProfitAbs, TotalProfitPct: src.Status.Bot.TotalProfitPct,
			LastPollTime: src.Status.Bot.LastPollTime, LastPollError: src.Status.Bot.LastPollError,
		}
		dst.Status.Bot = &bot
	}
}
