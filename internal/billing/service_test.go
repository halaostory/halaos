package billing

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

func newTestService(mockDB *testutil.MockDBTX) *Service {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(queries, logger)
}

// tokenBalanceScanValues returns values in the exact scan order for TokenBalance
// (8 fields: ID, CompanyID, Balance, TotalPurchased, TotalGranted, TotalConsumed, FreeTierGrantedAt, UpdatedAt).
func tokenBalanceScanValues(balance int64) []interface{} {
	return []interface{}{
		int64(1),             // ID
		int64(1),             // CompanyID
		balance,              // Balance
		int64(0),             // TotalPurchased
		int64(0),             // TotalGranted
		int64(0),             // TotalConsumed
		pgtype.Timestamptz{}, // FreeTierGrantedAt
		time.Now(),           // UpdatedAt
	}
}

// tokenTransactionScanValues returns values in the exact scan order for TokenTransaction
// (10 fields: ID, CompanyID, UserID, Type, Amount, BalanceAfter, AgentSlug, Description, Metadata, CreatedAt).
func tokenTransactionScanValues(txnType string, amount, balanceAfter int64) []interface{} {
	return []interface{}{
		int64(1),       // ID
		int64(1),       // CompanyID
		int64(1),       // UserID
		txnType,        // Type
		amount,         // Amount
		balanceAfter,   // BalanceAfter
		(*string)(nil), // AgentSlug
		(*string)(nil), // Description
		[]byte(nil),    // Metadata
		time.Now(),     // CreatedAt
	}
}

// tokenPackageScanValues returns values in the exact scan order for TokenPackage
// (8 fields: ID, Slug, Name, Tokens, PricePhp, IsActive, SortOrder, CreatedAt).
func tokenPackageScanValues() []interface{} {
	return []interface{}{
		int64(1),       // ID
		"starter",      // Slug
		"Starter Pack", // Name
		int64(1000),    // Tokens
		pgtype.Numeric{Int: big.NewInt(49900), Exp: -2, Valid: true}, // PricePhp (499.00)
		true,      // IsActive
		int32(1),  // SortOrder
		time.Now(), // CreatedAt
	}
}

// --- CheckBalance Tests ---

func TestCheckBalance_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(500)...))

	balance, err := svc.CheckBalance(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 500 {
		t.Fatalf("expected balance 500, got %d", balance)
	}
}

func TestCheckBalance_NoRows_ReturnsZero(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	balance, err := svc.CheckBalance(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 0 {
		t.Fatalf("expected balance 0, got %d", balance)
	}
}

func TestCheckBalance_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("connection failed")))

	_, err := svc.CheckBalance(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DeductTokens Tests ---

func TestDeductTokens_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// DeductTokenBalance returns updated balance
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(400)...))
	// InsertTokenTransaction succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("consumption", -100, 400)...))

	err := svc.DeductTokens(context.Background(), 1, 1, 100, "hr-agent", "Used for analysis")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeductTokens_ZeroAmount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	err := svc.DeductTokens(context.Background(), 1, 1, 0, "", "")
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}
}

func TestDeductTokens_NegativeAmount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	err := svc.DeductTokens(context.Background(), 1, 1, -10, "", "")
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

func TestDeductTokens_InsufficientBalance(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// DeductTokenBalance returns ErrNoRows when balance < amount
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	err := svc.DeductTokens(context.Background(), 1, 1, 100, "", "")
	if err != ErrInsufficientBalance {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestDeductTokens_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db down")))

	err := svc.DeductTokens(context.Background(), 1, 1, 100, "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err == ErrInsufficientBalance {
		t.Fatal("expected generic DB error, not ErrInsufficientBalance")
	}
}

func TestDeductTokens_TransactionInsertError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// DeductTokenBalance succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(400)...))
	// InsertTokenTransaction fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("insert failed")))

	err := svc.DeductTokens(context.Background(), 1, 1, 100, "", "")
	if err == nil {
		t.Fatal("expected error from transaction insert, got nil")
	}
}

func TestDeductTokens_EmptyAgentSlugAndDesc(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(400)...))
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("consumption", -100, 400)...))

	err := svc.DeductTokens(context.Background(), 1, 1, 100, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CreditTokens Tests ---

func TestCreditTokens_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// CreditTokenBalance succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(600)...))
	// InsertTokenTransaction succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("purchase", 100, 600)...))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "purchase", "Purchased starter pack")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreditTokens_Grant(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(200)...))
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("grant", 200, 200)...))

	err := svc.CreditTokens(context.Background(), 1, 1, 200, "grant", "Free monthly tokens")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreditTokens_ZeroAmount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	err := svc.CreditTokens(context.Background(), 1, 1, 0, "grant", "")
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}
}

func TestCreditTokens_NegativeAmount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	err := svc.CreditTokens(context.Background(), 1, 1, -50, "grant", "")
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

func TestCreditTokens_NoBalanceRow_EnsureAndRetry(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// First CreditTokenBalance returns ErrNoRows (no balance row)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// EnsureTokenBalance succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(0)...))
	// Retry CreditTokenBalance succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(100)...))
	// InsertTokenTransaction succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("grant", 100, 100)...))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "grant", "Initial grant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreditTokens_EnsureError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// First CreditTokenBalance returns ErrNoRows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// EnsureTokenBalance fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("ensure failed")))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "grant", "")
	if err == nil {
		t.Fatal("expected error from ensure, got nil")
	}
}

func TestCreditTokens_RetryAfterEnsure_Fails(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// First CreditTokenBalance returns ErrNoRows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// EnsureTokenBalance succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(0)...))
	// Retry CreditTokenBalance fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("credit still failed")))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "grant", "")
	if err == nil {
		t.Fatal("expected error from retry credit, got nil")
	}
}

func TestCreditTokens_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// CreditTokenBalance returns a non-ErrNoRows error
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("connection reset")))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "purchase", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreditTokens_TransactionInsertError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(600)...))
	// InsertTokenTransaction fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("insert failed")))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "purchase", "")
	if err == nil {
		t.Fatal("expected error from transaction insert, got nil")
	}
}

func TestCreditTokens_EmptyDescription(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(600)...))
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("grant", 100, 600)...))

	err := svc.CreditTokens(context.Background(), 1, 1, 100, "grant", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PurchaseTokens Tests ---

func TestPurchaseTokens_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// GetTokenPackage returns a valid package
	mockDB.OnQueryRow(testutil.NewRow(tokenPackageScanValues()...))
	// CreditTokenBalance succeeds (inside CreditTokens)
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(1000)...))
	// InsertTokenTransaction succeeds (inside CreditTokens)
	mockDB.OnQueryRow(testutil.NewRow(tokenTransactionScanValues("purchase", 1000, 1000)...))
	// GetTokenBalance for CheckBalance after purchase
	mockDB.OnQueryRow(testutil.NewRow(tokenBalanceScanValues(1000)...))

	result, err := svc.PurchaseTokens(context.Background(), 1, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PackageName != "Starter Pack" {
		t.Fatalf("expected PackageName 'Starter Pack', got %q", result.PackageName)
	}
	if result.Tokens != 1000 {
		t.Fatalf("expected 1000 tokens, got %d", result.Tokens)
	}
	if result.NewBalance != 1000 {
		t.Fatalf("expected new balance 1000, got %d", result.NewBalance)
	}
}

func TestPurchaseTokens_PackageNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	result, err := svc.PurchaseTokens(context.Background(), 1, 1, 999)
	if err == nil {
		t.Fatal("expected error for missing package, got nil")
	}
	if result != nil {
		t.Fatal("expected nil result for missing package")
	}
}

func TestPurchaseTokens_PackageDBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db timeout")))

	_, err := svc.PurchaseTokens(context.Background(), 1, 1, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPurchaseTokens_CreditFails(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	// GetTokenPackage succeeds
	mockDB.OnQueryRow(testutil.NewRow(tokenPackageScanValues()...))
	// CreditTokenBalance fails (inside CreditTokens)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("credit failed")))

	_, err := svc.PurchaseTokens(context.Background(), 1, 1, 1)
	if err == nil {
		t.Fatal("expected error from credit, got nil")
	}
}

// --- CalculateTokenCostStatic Tests ---

func TestCalculateTokenCostStatic(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int
		outputTokens int
		multiplier   float64
		expected     int64
	}{
		{
			name:         "basic calculation",
			inputTokens:  100,
			outputTokens: 50,
			multiplier:   1.0,
			expected:     25, // ceil(100*0.1 + 50*0.3) = ceil(10 + 15) = 25
		},
		{
			name:         "zero tokens",
			inputTokens:  0,
			outputTokens: 0,
			multiplier:   1.0,
			expected:     0,
		},
		{
			name:         "with multiplier",
			inputTokens:  100,
			outputTokens: 100,
			multiplier:   2.0,
			expected:     80, // ceil((100*0.1 + 100*0.3) * 2.0) = ceil(40*2) = 80
		},
		{
			name:         "zero multiplier defaults to 1.0",
			inputTokens:  100,
			outputTokens: 100,
			multiplier:   0,
			expected:     40, // ceil((100*0.1 + 100*0.3) * 1.0) = ceil(40) = 40
		},
		{
			name:         "negative multiplier defaults to 1.0",
			inputTokens:  100,
			outputTokens: 100,
			multiplier:   -1.0,
			expected:     40,
		},
		{
			name:         "fractional result rounds up",
			inputTokens:  1,
			outputTokens: 1,
			multiplier:   1.0,
			expected:     1, // ceil(0.1 + 0.3) = ceil(0.4) = 1
		},
		{
			name:         "large values",
			inputTokens:  10000,
			outputTokens: 5000,
			multiplier:   1.5,
			expected:     3750, // ceil((10000*0.1 + 5000*0.3) * 1.5) = ceil(2500*1.5) = 3750
		},
		{
			name:         "only input tokens",
			inputTokens:  200,
			outputTokens: 0,
			multiplier:   1.0,
			expected:     20, // ceil(200*0.1 + 0) = 20
		},
		{
			name:         "only output tokens",
			inputTokens:  0,
			outputTokens: 200,
			multiplier:   1.0,
			expected:     60, // ceil(0 + 200*0.3) = 60
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateTokenCostStatic(tt.inputTokens, tt.outputTokens, tt.multiplier)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestCalculateTokenCost_MethodForm verifies the method form delegates to the static function.
func TestCalculateTokenCost_MethodForm(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	svc := newTestService(mockDB)

	result := svc.CalculateTokenCost(100, 50, 1.0)
	expected := CalculateTokenCostStatic(100, 50, 1.0)
	if result != expected {
		t.Errorf("method form returned %d, static returned %d", result, expected)
	}
}

// --- NewService Tests ---

func TestNewService(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(queries, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.queries == nil {
		t.Fatal("expected non-nil queries")
	}
	if svc.logger == nil {
		t.Fatal("expected non-nil logger")
	}
}
