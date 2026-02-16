package server

import (
	"net/http"
	"strconv"

	"github.com/discuitnet/discuit/core"
	"github.com/discuitnet/discuit/internal/httperr"
	"github.com/discuitnet/discuit/internal/uid"
	"github.com/gorilla/mux"
)

// createDonation handles POST /api/communities/{communityId}/donate
// Creates a new donation to a community
func (s *Server) createDonation(w *responseWriter, r *request) error {
	if !r.loggedIn {
		return errNotLoggedIn
	}

	// Get community ID from URL
	vars := mux.Vars(r.req)
	communityIDStr := vars["communityId"]
	communityID, err := uid.FromString(communityIDStr)
	if err != nil {
		return httperr.NewBadRequest("invalid_community_id", "Invalid community ID")
	}

	// Parse request body
	reqBody := struct {
		Amount    int    `json:"amount"`    // Amount in cents
		Currency  string `json:"currency"`  // Currency code (e.g., "USD")
		Message   string `json:"message"`   // Optional message
		Anonymous bool   `json:"anonymous"` // Whether to donate anonymously
	}{}

	if err := r.unmarshalJSONBody(&reqBody); err != nil {
		return err
	}

	// Validate amount
	if reqBody.Amount <= 0 {
		return httperr.NewBadRequest("invalid_amount", "Amount must be greater than 0")
	}

	// Set default currency if not provided
	if reqBody.Currency == "" {
		reqBody.Currency = "USD"
	}

	// Create the donation
	donation, err := core.CreateDonation(
		r.ctx,
		s.db,
		*r.viewer,
		communityID,
		reqBody.Amount,
		reqBody.Currency,
		reqBody.Message,
		reqBody.Anonymous,
	)
	if err != nil {
		return err
	}

	// In a real implementation, you would integrate with a payment processor here
	// For now, we'll mark it as pending and return the donation
	// The payment processor would call back to update the status

	return w.writeJSON(donation)
}

// completeDonation handles POST /api/donations/{donationId}/complete
// Marks a donation as completed (would be called by payment processor webhook)
func (s *Server) completeDonation(w *responseWriter, r *request) error {
	// This would typically require admin privileges or be called by a webhook
	if !r.loggedIn {
		return errNotLoggedIn
	}

	// Get donation ID from URL
	vars := mux.Vars(r.req)
	donationIDStr := vars["donationId"]
	donationID, err := uid.FromString(donationIDStr)
	if err != nil {
		return httperr.NewBadRequest("invalid_donation_id", "Invalid donation ID")
	}

	// Parse request body
	reqBody := struct {
		PaymentMethod    string `json:"paymentMethod"`
		PaymentReference string `json:"paymentReference"`
	}{}

	if err := r.unmarshalJSONBody(&reqBody); err != nil {
		return err
	}

	// Update donation status
	err = core.UpdateDonationStatus(
		r.ctx,
		s.db,
		donationID,
		core.DonationStatusCompleted,
		reqBody.PaymentMethod,
		reqBody.PaymentReference,
	)
	if err != nil {
		return err
	}

	// Get updated donation
	donation, err := core.GetDonation(r.ctx, s.db, donationID)
	if err != nil {
		return err
	}

	return w.writeJSON(donation)
}

// getUserDonations handles GET /api/users/{username}/donations
// Gets donations made by a user
func (s *Server) getUserDonations(w *responseWriter, r *request) error {
	vars := mux.Vars(r.req)
	username := vars["username"]

	// Get the user
	user, err := core.GetUserByUsername(r.ctx, s.db, username, nil)
	if err != nil {
		return err
	}

	// Only allow users to see their own donations or admins
	if !r.loggedIn || (*r.viewer != user.ID && !s.isAdmin(r.ctx, *r.viewer)) {
		return httperr.NewForbidden("forbidden", "You don't have permission to view these donations")
	}

	// Get limit from query params
	limit := 20
	if limitStr := r.urlQueryParams().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	donations, err := core.GetUserDonations(r.ctx, s.db, user.ID, limit)
	if err != nil {
		return err
	}

	return w.writeJSON(donations)
}

// getCommunityDonations handles GET /api/communities/{communityId}/donations
// Gets donations for a community (only public/non-anonymous ones)
func (s *Server) getCommunityDonations(w *responseWriter, r *request) error {
	vars := mux.Vars(r.req)
	communityIDStr := vars["communityId"]
	communityID, err := uid.FromString(communityIDStr)
	if err != nil {
		return httperr.NewBadRequest("invalid_community_id", "Invalid community ID")
	}

	// Get limit from query params
	limit := 20
	if limitStr := r.urlQueryParams().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Only show non-anonymous donations to public
	// Admins or community mods could see all donations
	includeAnonymous := false
	if r.loggedIn {
		// Check if user is admin or community mod
		community, err := core.GetCommunityByID(r.ctx, s.db, communityID, r.viewer)
		if err != nil {
			return err
		}

		isModOrAdmin, err := userModOrAdmin(r.ctx, s.db, *r.viewer, community)
		if err != nil {
			return err
		}
		includeAnonymous = isModOrAdmin
	}

	donations, err := core.GetCommunityDonations(r.ctx, s.db, communityID, includeAnonymous, limit)
	if err != nil {
		return err
	}

	// Populate community and user info for non-anonymous donations
	for _, donation := range donations {
		if !donation.Anonymous {
			user, err := core.GetUser(r.ctx, s.db, donation.UserID, nil)
			if err == nil {
				donation.User = user
			}
		}
	}

	return w.writeJSON(donations)
}

// getCommunityDonationStats handles GET /api/communities/{communityId}/donations/stats
// Gets donation statistics for a community
func (s *Server) getCommunityDonationStats(w *responseWriter, r *request) error {
	vars := mux.Vars(r.req)
	communityIDStr := vars["communityId"]
	communityID, err := uid.FromString(communityIDStr)
	if err != nil {
		return httperr.NewBadRequest("invalid_community_id", "Invalid community ID")
	}

	stats, err := core.GetCommunityDonationStats(r.ctx, s.db, communityID)
	if err != nil {
		return err
	}

	return w.writeJSON(stats)
}

// getTopSupporters handles GET /api/communities/{communityId}/supporters
// Gets top supporters for a community
func (s *Server) getTopSupporters(w *responseWriter, r *request) error {
	vars := mux.Vars(r.req)
	communityIDStr := vars["communityId"]
	communityID, err := uid.FromString(communityIDStr)
	if err != nil {
		return httperr.NewBadRequest("invalid_community_id", "Invalid community ID")
	}

	// Get limit from query params
	limit := 10
	if limitStr := r.urlQueryParams().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	supporters, err := core.GetTopSupporters(r.ctx, s.db, communityID, limit)
	if err != nil {
		return err
	}

	// Populate user info
	for _, supporter := range supporters {
		user, err := core.GetUser(r.ctx, s.db, supporter.UserID, nil)
		if err == nil {
			supporter.User = user
		}
	}

	return w.writeJSON(supporters)
}

// isAdmin checks if a user is an admin
func (s *Server) isAdmin(ctx context.Context, userID uid.ID) bool {
	user, err := core.GetUser(ctx, s.db, userID, nil)
	if err != nil {
		return false
	}
	return user.Admin
}
