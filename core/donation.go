package core

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/discuitnet/discuit/internal/httperr"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/uid"
)

// DonationStatus represents the status of a donation
type DonationStatus string

const (
	DonationStatusPending   DonationStatus = "pending"
	DonationStatusCompleted DonationStatus = "completed"
	DonationStatusFailed    DonationStatus = "failed"
	DonationStatusRefunded  DonationStatus = "refunded"
)

// Donation represents a donation to a community
type Donation struct {
	ID               uid.ID         `json:"id"`
	UserID           uid.ID         `json:"userId"`
	CommunityID      uid.ID         `json:"communityId"`
	Amount           int            `json:"amount"` // Amount in cents
	Currency         string         `json:"currency"`
	Status           DonationStatus `json:"status"`
	PaymentMethod    string         `json:"paymentMethod,omitempty"`
	PaymentReference string         `json:"paymentReference,omitempty"`
	Message          msql.NullString `json:"message,omitempty"`
	Anonymous        bool           `json:"anonymous"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`

	// Populated on demand
	User      *User      `json:"user,omitempty"`
	Community *Community `json:"community,omitempty"`
}

// CommunityDonationStats represents aggregated donation statistics for a community
type CommunityDonationStats struct {
	CommunityID    uid.ID        `json:"communityId"`
	TotalAmount    int           `json:"totalAmount"` // Total amount raised in cents
	DonorCount     int           `json:"donorCount"`
	DonationCount  int           `json:"donationCount"`
	LastDonationAt msql.NullTime `json:"lastDonationAt,omitempty"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

// DonationSupporter represents a supporter of a community
type DonationSupporter struct {
	ID              int       `json:"id"`
	CommunityID     uid.ID    `json:"communityId"`
	UserID          uid.ID    `json:"userId"`
	TotalDonated    int       `json:"totalDonated"` // Total donated by this user in cents
	DonationCount   int       `json:"donationCount"`
	FirstDonationAt time.Time `json:"firstDonationAt"`
	LastDonationAt  time.Time `json:"lastDonationAt"`

	// Populated on demand
	User *User `json:"user,omitempty"`
}

// CreateDonation creates a new donation record
func CreateDonation(ctx context.Context, db *sql.DB, userID, communityID uid.ID, amount int, currency, message string, anonymous bool) (*Donation, error) {
	if amount <= 0 {
		return nil, httperr.NewBadRequest("invalid_amount", "Donation amount must be greater than 0")
	}

	if currency == "" {
		currency = "USD"
	}

	// Verify community exists
	community, err := GetCommunityByID(ctx, db, communityID, nil)
	if err != nil {
		return nil, err
	}
	if community.DeletedAt.Valid {
		return nil, httperr.NewBadRequest("community_deleted", "Cannot donate to deleted community")
	}

	donation := &Donation{
		ID:          uid.New(),
		UserID:      userID,
		CommunityID: communityID,
		Amount:      amount,
		Currency:    currency,
		Status:      DonationStatusPending,
		Anonymous:   anonymous,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if message != "" {
		donation.Message = msql.NewNullString(message)
	}

	query := `INSERT INTO donations (id, user_id, community_id, amount, currency, status, message, anonymous, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.ExecContext(ctx, query,
		donation.ID,
		donation.UserID,
		donation.CommunityID,
		donation.Amount,
		donation.Currency,
		donation.Status,
		donation.Message,
		donation.Anonymous,
		donation.CreatedAt,
		donation.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return donation, nil
}

// UpdateDonationStatus updates the status of a donation
func UpdateDonationStatus(ctx context.Context, db *sql.DB, donationID uid.ID, status DonationStatus, paymentMethod, paymentReference string) error {
	query := `UPDATE donations SET status = ?, payment_method = ?, payment_reference = ?, updated_at = NOW() WHERE id = ?`
	result, err := db.ExecContext(ctx, query, status, paymentMethod, paymentReference, donationID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return httperr.NewNotFound("donation_not_found", "Donation not found")
	}

	// If donation is completed, update statistics
	if status == DonationStatusCompleted {
		if err := updateDonationStatistics(ctx, db, donationID); err != nil {
			return err
		}
	}

	return nil
}

// updateDonationStatistics updates the aggregated statistics for a donation
func updateDonationStatistics(ctx context.Context, db *sql.DB, donationID uid.ID) error {
	// Get the donation
	donation, err := GetDonation(ctx, db, donationID)
	if err != nil {
		return err
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update or insert community donation stats
	statsQuery := `
		INSERT INTO community_donation_stats (community_id, total_amount, donor_count, donation_count, last_donation_at)
		VALUES (?, ?, 1, 1, NOW())
		ON DUPLICATE KEY UPDATE
			total_amount = total_amount + VALUES(total_amount),
			donation_count = donation_count + 1,
			last_donation_at = NOW()
	`
	_, err = tx.ExecContext(ctx, statsQuery, donation.CommunityID, donation.Amount)
	if err != nil {
		return err
	}

	// Update donor count separately (only increment if new donor)
	donorCountQuery := `
		UPDATE community_donation_stats 
		SET donor_count = (
			SELECT COUNT(DISTINCT user_id) 
			FROM donations 
			WHERE community_id = ? AND status = ?
		)
		WHERE community_id = ?
	`
	_, err = tx.ExecContext(ctx, donorCountQuery, donation.CommunityID, DonationStatusCompleted, donation.CommunityID)
	if err != nil {
		return err
	}

	// Update or insert donation supporter
	supporterQuery := `
		INSERT INTO donation_supporters (community_id, user_id, total_donated, donation_count, first_donation_at, last_donation_at)
		VALUES (?, ?, ?, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			total_donated = total_donated + VALUES(total_donated),
			donation_count = donation_count + 1,
			last_donation_at = NOW()
	`
	_, err = tx.ExecContext(ctx, supporterQuery, donation.CommunityID, donation.UserID, donation.Amount)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetDonation retrieves a donation by ID
func GetDonation(ctx context.Context, db *sql.DB, donationID uid.ID) (*Donation, error) {
	query := `SELECT id, user_id, community_id, amount, currency, status, payment_method, payment_reference, message, anonymous, created_at, updated_at
		FROM donations WHERE id = ?`

	donation := &Donation{}
	var paymentMethod, paymentReference sql.NullString

	err := db.QueryRowContext(ctx, query, donationID).Scan(
		&donation.ID,
		&donation.UserID,
		&donation.CommunityID,
		&donation.Amount,
		&donation.Currency,
		&donation.Status,
		&paymentMethod,
		&paymentReference,
		&donation.Message,
		&donation.Anonymous,
		&donation.CreatedAt,
		&donation.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, httperr.NewNotFound("donation_not_found", "Donation not found")
	}
	if err != nil {
		return nil, err
	}

	if paymentMethod.Valid {
		donation.PaymentMethod = paymentMethod.String
	}
	if paymentReference.Valid {
		donation.PaymentReference = paymentReference.String
	}

	return donation, nil
}

// GetUserDonations retrieves donations made by a user
func GetUserDonations(ctx context.Context, db *sql.DB, userID uid.ID, limit int) ([]*Donation, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT id, user_id, community_id, amount, currency, status, payment_method, payment_reference, message, anonymous, created_at, updated_at
		FROM donations WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`

	rows, err := db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	donations := []*Donation{}
	for rows.Next() {
		donation := &Donation{}
		var paymentMethod, paymentReference sql.NullString

		err := rows.Scan(
			&donation.ID,
			&donation.UserID,
			&donation.CommunityID,
			&donation.Amount,
			&donation.Currency,
			&donation.Status,
			&paymentMethod,
			&paymentReference,
			&donation.Message,
			&donation.Anonymous,
			&donation.CreatedAt,
			&donation.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if paymentMethod.Valid {
			donation.PaymentMethod = paymentMethod.String
		}
		if paymentReference.Valid {
			donation.PaymentReference = paymentReference.String
		}

		donations = append(donations, donation)
	}

	return donations, rows.Err()
}

// GetCommunityDonations retrieves donations for a community
func GetCommunityDonations(ctx context.Context, db *sql.DB, communityID uid.ID, includeAnonymous bool, limit int) ([]*Donation, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT id, user_id, community_id, amount, currency, status, payment_method, payment_reference, message, anonymous, created_at, updated_at
		FROM donations WHERE community_id = ? AND status = ?`

	if !includeAnonymous {
		query += ` AND anonymous = false`
	}

	query += ` ORDER BY created_at DESC LIMIT ?`

	rows, err := db.QueryContext(ctx, query, communityID, DonationStatusCompleted, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	donations := []*Donation{}
	for rows.Next() {
		donation := &Donation{}
		var paymentMethod, paymentReference sql.NullString

		err := rows.Scan(
			&donation.ID,
			&donation.UserID,
			&donation.CommunityID,
			&donation.Amount,
			&donation.Currency,
			&donation.Status,
			&paymentMethod,
			&paymentReference,
			&donation.Message,
			&donation.Anonymous,
			&donation.CreatedAt,
			&donation.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if paymentMethod.Valid {
			donation.PaymentMethod = paymentMethod.String
		}
		if paymentReference.Valid {
			donation.PaymentReference = paymentReference.String
		}

		donations = append(donations, donation)
	}

	return donations, rows.Err()
}

// GetCommunityDonationStats retrieves donation statistics for a community
func GetCommunityDonationStats(ctx context.Context, db *sql.DB, communityID uid.ID) (*CommunityDonationStats, error) {
	query := `SELECT community_id, total_amount, donor_count, donation_count, last_donation_at, updated_at
		FROM community_donation_stats WHERE community_id = ?`

	stats := &CommunityDonationStats{}
	err := db.QueryRowContext(ctx, query, communityID).Scan(
		&stats.CommunityID,
		&stats.TotalAmount,
		&stats.DonorCount,
		&stats.DonationCount,
		&stats.LastDonationAt,
		&stats.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return zero stats if no donations yet
		return &CommunityDonationStats{
			CommunityID:   communityID,
			TotalAmount:   0,
			DonorCount:    0,
			DonationCount: 0,
			UpdatedAt:     time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetTopSupporters retrieves the top supporters for a community
func GetTopSupporters(ctx context.Context, db *sql.DB, communityID uid.ID, limit int) ([]*DonationSupporter, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `SELECT id, community_id, user_id, total_donated, donation_count, first_donation_at, last_donation_at
		FROM donation_supporters 
		WHERE community_id = ? 
		ORDER BY total_donated DESC 
		LIMIT ?`

	rows, err := db.QueryContext(ctx, query, communityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	supporters := []*DonationSupporter{}
	for rows.Next() {
		supporter := &DonationSupporter{}
		err := rows.Scan(
			&supporter.ID,
			&supporter.CommunityID,
			&supporter.UserID,
			&supporter.TotalDonated,
			&supporter.DonationCount,
			&supporter.FirstDonationAt,
			&supporter.LastDonationAt,
		)
		if err != nil {
			return nil, err
		}

		supporters = append(supporters, supporter)
	}

	return supporters, rows.Err()
}

// FormatDonationAmount formats a donation amount (in cents) to a display string
func FormatDonationAmount(amount int, currency string) string {
	switch currency {
	case "USD", "EUR", "GBP", "CAD", "AUD":
		return fmt.Sprintf("$%.2f", float64(amount)/100.0)
	default:
		return fmt.Sprintf("%d %s", amount, currency)
	}
}
