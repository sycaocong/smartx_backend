package futures

import (
	"testing"
)

func TestNewFuturesOrder(t *testing.T) {
	order := NewFuturesOrder("BTCUSDT_PERP", Buy, FuturesLimitOrder,
		94000.0, 0.1, 10, IsolatedMargin, Long, "user1", "client123")

	if order.OrderID == "" {
		t.Error("Order ID should not be empty")
	}
	if order.Symbol != "BTCUSDT_PERP" {
		t.Errorf("Expected symbol BTCUSDT_PERP, got %s", order.Symbol)
	}
	if order.Side != Buy {
		t.Errorf("Expected side Buy, got %d", order.Side)
	}
	if order.Leverage != 10 {
		t.Errorf("Expected leverage 10, got %d", order.Leverage)
	}
	if order.PositionSide != Long {
		t.Errorf("Expected position side Long, got %d", order.PositionSide)
	}
}

func TestOrderFill(t *testing.T) {
	order := NewFuturesOrder("BTCUSDT_PERP", Buy, FuturesLimitOrder,
		94000.0, 1.0, 10, IsolatedMargin, Long, "user1", "client123")

	order.Fill(0.5, 94000.0)
	if order.FilledQuantity != 0.5 {
		t.Errorf("Expected filled quantity 0.5, got %f", order.FilledQuantity)
	}
	if order.AvgFillPrice != 94000.0 {
		t.Errorf("Expected avg fill price 94000.0, got %f", order.AvgFillPrice)
	}

	order.Fill(0.5, 94100.0)
	if order.FilledQuantity != 1.0 {
		t.Errorf("Expected filled quantity 1.0, got %f", order.FilledQuantity)
	}
	expectedAvg := (94000.0*0.5 + 94100.0*0.5) / 1.0
	if order.AvgFillPrice != expectedAvg {
		t.Errorf("Expected avg fill price %f, got %f", expectedAvg, order.AvgFillPrice)
	}
	if !order.IsDone() {
		t.Error("Order should be done")
	}
}

func TestOrderCancel(t *testing.T) {
	order := NewFuturesOrder("BTCUSDT_PERP", Buy, FuturesLimitOrder,
		94000.0, 1.0, 10, IsolatedMargin, Long, "user1", "client123")

	order.Cancel()
	if !order.IsDone() {
		t.Error("Order should be done after cancel")
	}
}

func TestPositionUpdate(t *testing.T) {
	position := NewPosition("user1", "BTCUSDT_PERP", Long, 1.0, 94000.0, 9400.0, 10, IsolatedMargin)

	position.UpdateMarkPrice(94500.0)
	if position.MarkPrice != 94500.0 {
		t.Errorf("Expected mark price 94500.0, got %f", position.MarkPrice)
	}

	position.UpdateUnrealizedPNL()
	expectedPNL := 1.0 * (94500.0 - 94000.0)
	if position.UnrealizedPNL != expectedPNL {
		t.Errorf("Expected unrealized PNL %f, got %f", expectedPNL, position.UnrealizedPNL)
	}
}

func TestPositionClose(t *testing.T) {
	position := NewPosition("user1", "BTCUSDT_PERP", Long, 1.0, 94000.0, 9400.0, 10, IsolatedMargin)

	position.Close(94500.0)
	if position.Status != PositionClosed {
		t.Errorf("Expected status PositionClosed, got %d", position.Status)
	}
	if position.RealizedPNL != 500.0 {
		t.Errorf("Expected realized PNL 500.0, got %f", position.RealizedPNL)
	}
}

func TestPositionLiquidate(t *testing.T) {
	position := NewPosition("user1", "BTCUSDT_PERP", Long, 1.0, 94000.0, 9400.0, 10, IsolatedMargin)

	position.Liquidate(93000.0)
	if position.Status != PositionLiquidated {
		t.Errorf("Expected status PositionLiquidated, got %d", position.Status)
	}
	if position.RealizedPNL != -1000.0 {
		t.Errorf("Expected realized PNL -1000.0, got %f", position.RealizedPNL)
	}
}

func TestPositionAddFundingFee(t *testing.T) {
	position := NewPosition("user1", "BTCUSDT_PERP", Long, 1.0, 94000.0, 9400.0, 10, IsolatedMargin)

	position.AddFundingFee(10.0)
	if position.FundingFee != 10.0 {
		t.Errorf("Expected funding fee 10.0, got %f", position.FundingFee)
	}
	if position.Margin != 9390.0 {
		t.Errorf("Expected margin 9390.0, got %f", position.Margin)
	}
}