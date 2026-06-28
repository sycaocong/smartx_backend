package risk

import (
	"testing"

	"github.com/smartx/matching-engine/engine/futures"
)

func TestCalculateInitialMargin(t *testing.T) {
	margin := CalculateInitialMargin(94000.0, 1.0, 10)
	expected := 9400.0
	if margin != expected {
		t.Errorf("Expected initial margin %f, got %f", expected, margin)
	}
}

func TestCalculateMaintenanceMargin(t *testing.T) {
	margin := CalculateMaintenanceMargin(94000.0, 1.0)
	expected := 94000.0 * 0.005
	if margin != expected {
		t.Errorf("Expected maintenance margin %f, got %f", expected, margin)
	}
}

func TestCalculateLiquidationPrice(t *testing.T) {
	position := &futures.Position{
		EntryPrice: 94000.0,
		Leverage:   10,
		Side:       futures.Long,
	}
	liqPrice := CalculateLiquidationPrice(position)
	if liqPrice <= 0 {
		t.Errorf("Expected positive liquidation price, got %f", liqPrice)
	}

	position.Side = futures.Short
	liqPrice = CalculateLiquidationPrice(position)
	if liqPrice <= 0 {
		t.Errorf("Expected positive liquidation price for short, got %f", liqPrice)
	}
}

func TestCheckMarginCall(t *testing.T) {
	position := futures.NewPosition("user1", "BTCUSDT_PERP", futures.Long, 1.0, 94000.0, 9400.0, 10, futures.IsolatedMargin)

	if CheckMarginCall(position, 94000.0) {
		t.Error("Should not trigger margin call at entry price")
	}

	if !CheckMarginCall(position, 84000.0) {
		t.Error("Should trigger margin call at 84000.0")
	}
}

func TestCheckLiquidation(t *testing.T) {
	position := futures.NewPosition("user1", "BTCUSDT_PERP", futures.Long, 1.0, 94000.0, 9400.0, 10, futures.IsolatedMargin)
	position.LiquidationPrice = 93000.0

	if CheckLiquidation(position, 93500.0) {
		t.Error("Should not trigger liquidation above liquidation price")
	}

	if !CheckLiquidation(position, 93000.0) {
		t.Error("Should trigger liquidation at liquidation price")
	}

	position.Side = futures.Short
	position.LiquidationPrice = 95000.0
	if CheckLiquidation(position, 94500.0) {
		t.Error("Should not trigger liquidation below liquidation price for short")
	}
	if !CheckLiquidation(position, 95000.0) {
		t.Error("Should trigger liquidation at liquidation price for short")
	}
}