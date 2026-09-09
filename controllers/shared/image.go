package shared

// DefaultFreqtradeImage is used when a controller's own DefaultImage field
// is unset (also cmd/main.go's --default-freqtrade-image flag default, so
// both stay in sync from this one definition), and by both
// controllers/tradebot and controllers/backtest (P6-1) - a Backtest run
// needs the identical default a live bot does. Digest-pinned (P3-3) rather
// than a floating tag like the old freqtradeorg/freqtrade:stable, so a pod
// restart can never silently change what version of freqtrade is running.
// Update it deliberately, as its own reviewable change, when freqtrade
// ships a version worth moving to; verified this exact digest starts
// cleanly under the P3-3 restricted SecurityContext (both `trade` and
// `download-data`) before pinning it.
const DefaultFreqtradeImage = "freqtradeorg/freqtrade@sha256:" +
	"7031bca43ed7668ebf421725dd5016acade6ef88b0771db3e08c96e6d19a42db"
