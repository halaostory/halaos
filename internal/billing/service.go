package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/store"
)

// ErrInsufficientBalance is returned when a company does not have enough tokens.
var ErrInsufficientBalance = errors.New("insufficient token balance")

// PurchaseResult contains the outcome of a token purchase.
type PurchaseResult struct {
	PackageName string `json:"package_name"`
	Tokens      int64  `json:"tokens"`
	PricePHP    string `json:"price_php"`
	NewBalance  int64  `json:"new_balance"`
}

// Service manages token balances and transactions.
type Service struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewService creates a new billing service.
func NewService(queries *store.Queries, logger *slog.Logger) *Service {
	return &Service{
		queries: queries,
		logger:  logger,
	}
}

// CheckBalance returns the current token balance for a company.
// If no balance record exists, it returns 0.
func (s *Service) CheckBalance(ctx context.Context, companyID int64) (int64, error) {
	tb, err := s.queries.GetTokenBalance(ctx, companyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		s.logger.Error("failed to check token balance",
			"company_id", companyID,
			"error", err,
		)
		return 0, fmt.Errorf("check balance: %w", err)
	}
	return tb.Balance, nil
}

// DeductTokens atomically deducts tokens and writes a transaction record.
// Returns ErrInsufficientBalance if the company does not have enough tokens.
func (s *Service) DeductTokens(ctx context.Context, companyID, userID, amount int64, agentSlug, desc string) error {
	if amount <= 0 {
		return errors.New("deduction amount must be positive")
	}

	result, err := s.queries.DeductTokenBalance(ctx, store.DeductTokenBalanceParams{
		CompanyID: companyID,
		Balance:   amount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientBalance
		}
		s.logger.Error("failed to deduct token balance",
			"company_id", companyID,
			"amount", amount,
			"error", err,
		)
		return fmt.Errorf("deduct tokens: %w", err)
	}

	// Record the consumption transaction with a negative amount.
	var agentSlugPtr *string
	if agentSlug != "" {
		agentSlugPtr = &agentSlug
	}
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	_, txnErr := s.queries.InsertTokenTransaction(ctx, store.InsertTokenTransactionParams{
		CompanyID:    companyID,
		UserID:       userID,
		Type:         "consumption",
		Amount:       -amount,
		BalanceAfter: result.Balance,
		AgentSlug:    agentSlugPtr,
		Description:  descPtr,
	})
	if txnErr != nil {
		s.logger.Error("failed to insert deduction transaction",
			"company_id", companyID,
			"amount", amount,
			"error", txnErr,
		)
		return fmt.Errorf("insert deduction transaction: %w", txnErr)
	}

	s.logger.Info("tokens deducted",
		"company_id", companyID,
		"user_id", userID,
		"amount", amount,
		"balance_after", result.Balance,
	)

	return nil
}

// CreditTokens adds tokens to a company's balance (for purchases or grants).
func (s *Service) CreditTokens(ctx context.Context, companyID, userID, amount int64, txnType, desc string) error {
	if amount <= 0 {
		return errors.New("credit amount must be positive")
	}

	isPurchase := txnType == "purchase"
	result, err := s.queries.CreditTokenBalance(ctx, store.CreditTokenBalanceParams{
		CompanyID: companyID,
		Balance:   amount,
		Column3:   isPurchase,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No balance row exists; ensure one is created then retry.
			if _, ensureErr := s.queries.EnsureTokenBalance(ctx, companyID); ensureErr != nil {
				s.logger.Error("failed to ensure token balance",
					"company_id", companyID,
					"error", ensureErr,
				)
				return fmt.Errorf("ensure token balance: %w", ensureErr)
			}
			result, err = s.queries.CreditTokenBalance(ctx, store.CreditTokenBalanceParams{
				CompanyID: companyID,
				Balance:   amount,
				Column3:   isPurchase,
			})
			if err != nil {
				s.logger.Error("failed to credit token balance after ensure",
					"company_id", companyID,
					"error", err,
				)
				return fmt.Errorf("credit tokens: %w", err)
			}
		} else {
			s.logger.Error("failed to credit token balance",
				"company_id", companyID,
				"amount", amount,
				"error", err,
			)
			return fmt.Errorf("credit tokens: %w", err)
		}
	}

	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	_, txnErr := s.queries.InsertTokenTransaction(ctx, store.InsertTokenTransactionParams{
		CompanyID:    companyID,
		UserID:       userID,
		Type:         txnType,
		Amount:       amount,
		BalanceAfter: result.Balance,
		Description:  descPtr,
	})
	if txnErr != nil {
		s.logger.Error("failed to insert credit transaction",
			"company_id", companyID,
			"amount", amount,
			"error", txnErr,
		)
		return fmt.Errorf("insert credit transaction: %w", txnErr)
	}

	s.logger.Info("tokens credited",
		"company_id", companyID,
		"user_id", userID,
		"type", txnType,
		"amount", amount,
		"balance_after", result.Balance,
	)

	return nil
}

// PurchaseTokens simulates purchasing a token package.
// MVP: no real payment gateway; tokens are credited immediately.
func (s *Service) PurchaseTokens(ctx context.Context, companyID, userID, packageID int64) (*PurchaseResult, error) {
	pkg, err := s.queries.GetTokenPackage(ctx, packageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("token package not found or inactive")
		}
		return nil, fmt.Errorf("get token package: %w", err)
	}

	// Format price from pgtype.Numeric.
	priceFloat, err := pkg.PricePhp.Float64Value()
	if err != nil {
		return nil, fmt.Errorf("parse package price: %w", err)
	}
	priceStr := fmt.Sprintf("%.2f", priceFloat.Float64)

	desc := fmt.Sprintf("Purchased %s (%s tokens) for PHP %s", pkg.Name, fmt.Sprint(pkg.Tokens), priceStr)
	if creditErr := s.CreditTokens(ctx, companyID, userID, pkg.Tokens, "purchase", desc); creditErr != nil {
		return nil, fmt.Errorf("credit purchased tokens: %w", creditErr)
	}

	// Read new balance after credit.
	newBalance, err := s.CheckBalance(ctx, companyID)
	if err != nil {
		// Non-fatal: purchase already succeeded; log and return best-effort.
		s.logger.Warn("failed to read balance after purchase", "error", err)
	}

	return &PurchaseResult{
		PackageName: pkg.Name,
		Tokens:      pkg.Tokens,
		PricePHP:    priceStr,
		NewBalance:  newBalance,
	}, nil
}

// CalculateTokenCost computes the token cost from LLM usage (method form for interface satisfaction).
func (s *Service) CalculateTokenCost(inputTokens, outputTokens int, multiplier float64) int64 {
	return CalculateTokenCostStatic(inputTokens, outputTokens, multiplier)
}

// CalculateTokenCostStatic computes the token cost from LLM usage.
// Formula: ceil((inputTokens * 0.1 + outputTokens * 0.3) * multiplier)
func CalculateTokenCostStatic(inputTokens, outputTokens int, multiplier float64) int64 {
	if multiplier <= 0 {
		multiplier = 1.0
	}
	cost := (float64(inputTokens)*0.1 + float64(outputTokens)*0.3) * multiplier
	return int64(math.Ceil(cost))
}
