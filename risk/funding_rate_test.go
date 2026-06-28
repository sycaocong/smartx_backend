package risk

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smartx/matching-engine/engine/futures"
)

func TestCalculateFundingRate(t *testing.T) {
	logger := zerolog.Nop()
	mpe := NewMarkPriceEngine("BTCUSDT_PERP", logger)
	mpe.SetExchangePrice("binance", 94000.0)

	marginEngine := NewMarginEngine()
	fre := NewFundingRateEngine("BTCUSDT_PERP", mpe, marginEngine, logger)

	mpe.SetExchangePrice("binance", 94000.0)
	mpe.CalculateMarkPrice(0, 8*time.Hour)

	rate := fre.CalculateFundingRate()
	if rate < -FundingRateLimit || rate > FundingRateLimit {
		t.Errorf("Funding rate %f exceeds limit %f", rate, FundingRateLimit)
	}
}

func TestSettleFunding(t *testing.T) {
	logger := zerolog.Nop()
	mpe := NewMarkPriceEngine("BTCUSDT_PERP", logger)
	mpe.SetExchangePrice("binance", 94000.0)
	mpe.CalculateMarkPrice(0, 8*time.Hour)

	marginEngine := NewMarginEngine()
	fre := NewFundingRateEngine("BTCUSDT_PERP", mpe, marginEngine, logger)

	position := futures.NewPosition("user1", "BTCUSDT_PERP", futures.Long, 1.0, 94000.0, 9400.0, 10, futures.IsolatedMargin)
	position.UpdateMarkPrice(94000.0)
	marginEngine.AddPosition(position)

	fre.CalculateFundingRate()
	settlements := fre.SettleFunding()

	if len(settlements) != 1 {
		t.Errorf("Expected 1 settlement, got %d", len(settlements))
	}
}