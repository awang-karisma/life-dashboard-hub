package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type FamiliesHandler struct {
	DB             *database.DB
	ReportsHandler *ReportsHandler
}

func NewFamiliesHandler(db *database.DB, reportsHandler *ReportsHandler) *FamiliesHandler {
	return &FamiliesHandler{
		DB:             db,
		ReportsHandler: reportsHandler,
	}
}

type CreateFamilyRequest struct {
	Name string `json:"name"`
}
type UpdateFamilyRequest struct {
	Name string `json:"name"`
}
type UpdateFamilySettingsRequest struct {
	MembersCanViewOthers *bool `json:"members_can_view_others"`
}



type InviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // parent, adult, child
}

type CreateChildRequest struct {
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type UpdateSharingRequest struct {
	Sharing string `json:"sharing"` // none, summary, detailed
}

type FamilyResponse struct {
	ID        uint                   `json:"id"`
	Name      string                 `json:"name"`
	CreatedBy uint                   `json:"created_by"`
	CreatedAt time.Time              `json:"created_at"`
	Role      string                 `json:"role,omitempty"`
	Sharing   string                 `json:"sharing,omitempty"`
	Members   []FamilyMemberResponse `json:"members,omitempty"`
}

type FamilyMemberResponse struct {
	UserID      uint      `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Sharing     string    `json:"sharing"`
	Status      string    `json:"status"`
	JoinedAt    time.Time `json:"joined_at"`
}

type InvitationResponse struct {
	ID           uint      `json:"id"`
	FamilyID     uint      `json:"family_id"`
	InvitedEmail string    `json:"invited_email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	InviteToken  string    `json:"invite_token,omitempty"`
}

// CreateFamily godoc
// @Summary      Create a new family group
// @Description  Creates a family and makes current user admin
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        body body CreateFamilyRequest true "Family details"
// @Success      201 {object} FamilyResponse
// @Failure      400 {object} ErrorResponse
// @Router       /families [post]
func (h *FamiliesHandler) CreateFamily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var req CreateFamilyRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "family name is required"})
	}

	var family models.Family
	var membership models.FamilyMembership

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var existingCount int64
		tx.Model(&models.FamilyMembership{}).Where("user_id = ? AND status = 'active'", userID).Count(&existingCount)
		if existingCount > 0 {
			return errors.New("already belongs to a family")
		}

		family = models.Family{
			Name:      req.Name,
			CreatedBy: userID,
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&family).Error; err != nil {
			return err
		}

		membership = models.FamilyMembership{
			FamilyID: family.ID,
			UserID:   userID,
			Role:     "admin",
			Sharing:  "none",
			Status:   "active",
			JoinedAt: time.Now().UTC(),
		}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}

		settings := models.FamilySettings{
			FamilyID:             family.ID,
			MembersCanViewOthers: false,
		}
		return tx.Create(&settings).Error
	})

	if err != nil {
		if err.Error() == "already belongs to a family" {
			return c.Status(fiber.StatusConflict).JSON(ErrorResponse{Error: "you already belong to a family"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create family"})
	}

	return c.Status(fiber.StatusCreated).JSON(FamilyResponse{
		ID:        family.ID,
		Name:      family.Name,
		CreatedBy: family.CreatedBy,
		CreatedAt: family.CreatedAt,
		Role:      "admin",
		Sharing:   "none",
	})
}

// ListFamilies godoc
// @Summary      List user's families
// @Description  Returns all families current user belongs to
// @Tags         families
// @Produce      json
// @Success      200 {array} FamilyResponse
// @Router       /families [get]
func (h *FamiliesHandler) ListFamilies(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var memberships []models.FamilyMembership
	if err := h.DB.Preload("Family").Where("user_id = ? AND status = 'active'", userID).Find(&memberships).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
	}

	result := make([]FamilyResponse, len(memberships))
	for i, m := range memberships {
		result[i] = FamilyResponse{
			ID:        m.Family.ID,
			Name:      m.Family.Name,
			CreatedBy: m.Family.CreatedBy,
			CreatedAt: m.Family.CreatedAt,
			Role:      m.Role,
			Sharing:   m.Sharing,
		}
	}

	return c.JSON(result)
}

// GetFamily godoc
// @Summary      Get family details
// @Description  Returns family information and member roster
// @Tags         families
// @Produce      json
// @Param        id path int true "Family ID"
// @Success      200 {object} FamilyResponse
// @Failure      403 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Router       /families/{id} [get]
func (h *FamiliesHandler) GetFamily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	// Verify membership
	var userMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&userMembership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "not a member of this family"})
	}

	var family models.Family
	if err := h.DB.First(&family, familyID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "family not found"})
	}

	var allMemberships []models.FamilyMembership
	if err := h.DB.Preload("User").Where("family_id = ? AND status = 'active'", familyID).Find(&allMemberships).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
	}

	members := make([]FamilyMemberResponse, len(allMemberships))
	for i, m := range allMemberships {
		members[i] = FamilyMemberResponse{
			UserID:      m.UserID,
			DisplayName: m.User.DisplayName,
			Email:       m.User.Email,
			Role:        m.Role,
			Sharing:     m.Sharing,
			Status:      m.Status,
			JoinedAt:    m.JoinedAt,
		}
	}

	return c.JSON(FamilyResponse{
		ID:        family.ID,
		Name:      family.Name,
		CreatedBy: family.CreatedBy,
		CreatedAt: family.CreatedAt,
		Role:      userMembership.Role,
		Sharing:   userMembership.Sharing,
		Members:   members,
	})
}
// UpdateFamily godoc
// @Summary      Update family details
// @Description  Admin updates family name
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        id path int true "Family ID"
// @Param        body body UpdateFamilyRequest true "Family details"
// @Success      200 {object} MessageResponse
// @Failure      400 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id} [patch]
func (h *FamiliesHandler) UpdateFamily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil || membership.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only family admins can update the family"})
	}

	var req UpdateFamilyRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "family name is required"})
	}

	if err := h.DB.Model(&models.Family{}).Where("id = ?", familyID).Update("name", req.Name).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update family"})
	}

	return c.JSON(MessageResponse{Message: "family updated"})
}

// DeleteFamily godoc
// @Summary      Delete family
// @Description  Admin deletes family and removes all memberships
// @Tags         families
// @Produce      json
// @Param        id path int true "Family ID"
// @Success      200 {object} MessageResponse
// @Failure      400 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id} [delete]
func (h *FamiliesHandler) DeleteFamily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil || membership.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only family admins can delete the family"})
	}

	now := time.Now().UTC()
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.FamilyMembership{}).Where("family_id = ?", familyID).Updates(map[string]interface{}{
			"status":  "removed",
			"left_at": now,
		}).Error; err != nil {
			return err
		}

		if err := tx.Where("family_id = ?", familyID).Delete(&models.Invitation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("family_id = ?", familyID).Delete(&models.FamilySettings{}).Error; err != nil {
			return err
		}

		return tx.Delete(&models.Family{}, familyID).Error
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to delete family"})
	}

	return c.JSON(MessageResponse{Message: "family deleted"})
}

// GetFamilySettings godoc
// @Summary      Get family settings
// @Description  Get settings for a family (members only)
// @Tags         families
// @Produce      json
// @Param        id path int true "Family ID"
// @Success      200 {object} models.FamilySettings
// @Failure      400 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id}/settings [get]
func (h *FamiliesHandler) GetFamilySettings(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "not a member of this family"})
	}

	var settings models.FamilySettings
	if err := h.DB.Where("family_id = ?", familyID).First(&settings).Error; err != nil {
		settings = models.FamilySettings{
			FamilyID:             uint(familyID),
			MembersCanViewOthers: false,
		}
		_ = h.DB.Create(&settings)
	}

	return c.JSON(settings)
}

// UpdateFamilySettings godoc
// @Summary      Update family settings
// @Description  Admin updates family settings
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        id path int true "Family ID"
// @Param        body body UpdateFamilySettingsRequest true "Settings details"
// @Success      200 {object} MessageResponse
// @Failure      400 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id}/settings [patch]
func (h *FamiliesHandler) UpdateFamilySettings(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil || membership.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only family admins can update settings"})
	}

	var req UpdateFamilySettingsRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.MembersCanViewOthers == nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "members_can_view_others is required"})
	}

	var settings models.FamilySettings
	if err := h.DB.Where("family_id = ?", familyID).First(&settings).Error; err != nil {
		settings = models.FamilySettings{
			FamilyID:             uint(familyID),
			MembersCanViewOthers: *req.MembersCanViewOthers,
		}
		if err := h.DB.Create(&settings).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update settings"})
		}
	} else {
		settings.MembersCanViewOthers = *req.MembersCanViewOthers
		if err := h.DB.Save(&settings).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update settings"})
		}
	}

	return c.JSON(MessageResponse{Message: "family settings updated"})
}


// InviteMember godoc
// @Summary      Invite a member to the family
// @Description  Creates an invitation token (admin only)
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        id path int true "Family ID"
// @Param        body body InviteMemberRequest true "Invitation details"
// @Success      201 {object} InvitationResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id}/invitations [post]
func (h *FamiliesHandler) InviteMember(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	// Verify admin role
	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil || membership.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only family admins can invite members"})
	}

	var req InviteMemberRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "parent" && req.Role != "adult" && req.Role != "child" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "role must be parent, adult, or child"})
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to generate token"})
	}
	rawToken := base64.RawURLEncoding.EncodeToString(tokenBytes)

	hHash := sha256.New()
	hHash.Write([]byte(rawToken))
	tokenHash := hex.EncodeToString(hHash.Sum(nil))

	now := time.Now().UTC()
	invitation := models.Invitation{
		FamilyID:     uint(familyID),
		TokenHash:    tokenHash,
		InvitedEmail: strings.TrimSpace(strings.ToLower(req.Email)),
		Role:         req.Role,
		Status:       "pending",
		CreatedBy:    userID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(7 * 24 * time.Hour),
	}

	if err := h.DB.Create(&invitation).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create invitation"})
	}

	return c.Status(fiber.StatusCreated).JSON(InvitationResponse{
		ID:           invitation.ID,
		FamilyID:     invitation.FamilyID,
		InvitedEmail: invitation.InvitedEmail,
		Role:         invitation.Role,
		Status:       invitation.Status,
		ExpiresAt:    invitation.ExpiresAt,
		InviteToken:  rawToken,
	})
}

// AcceptInvitation godoc
// @Summary      Accept a family invitation
// @Description  Join family using invitation token
// @Tags         families
// @Produce      json
// @Param        token path string true "Invitation token"
// @Success      200 {object} MessageResponse
// @Failure      400 {object} ErrorResponse
// @Router       /invitations/{token}/accept [post]
func (h *FamiliesHandler) AcceptInvitation(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	token := c.Params("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "token required"})
	}

	hHash := sha256.New()
	hHash.Write([]byte(token))
	tokenHash := hex.EncodeToString(hHash.Sum(nil))

	var invitation models.Invitation
	if err := h.DB.Where("token_hash = ?", tokenHash).First(&invitation).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid invitation token"})
	}

	now := time.Now().UTC()
	if invitation.Status != "pending" || invitation.ExpiresAt.Before(now) {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invitation is expired or already used"})
	}

	// Verify accepting user's email matches invitation
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to look up user"})
	}
	if invitation.InvitedEmail != "" && strings.ToLower(user.Email) != strings.ToLower(invitation.InvitedEmail) {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "this invitation was sent to a different email address"})
	}

	var errAlreadyMember = errors.New("already a member of another family")

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// Create or activate membership
		var existing models.FamilyMembership
		var existingMembership models.FamilyMembership
		if err := tx.Where("user_id = ? AND status = 'active'", userID).First(&existingMembership).Error; err == nil {
			if existingMembership.FamilyID != invitation.FamilyID {
				return errAlreadyMember
			}
		}

		err := tx.Where("family_id = ? AND user_id = ?", invitation.FamilyID, userID).First(&existing).Error
		if err == nil {
			existing.Role = invitation.Role
			existing.Status = "active"
			existing.LeftAt = nil
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		} else {
			membership := models.FamilyMembership{
				FamilyID: invitation.FamilyID,
				UserID:   userID,
				Role:     invitation.Role,
				Sharing:  "none",
				Status:   "active",
				JoinedAt: now,
			}
			if err := tx.Create(&membership).Error; err != nil {
				return err
			}
		}

		invitation.Status = "accepted"
		invitation.AcceptedAt = &now
		return tx.Save(&invitation).Error
	})

	if err != nil {
		if errors.Is(err, errAlreadyMember) {
			return c.Status(fiber.StatusConflict).JSON(ErrorResponse{Error: "you already belong to a family"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to accept invitation"})
	}

	return c.JSON(MessageResponse{Message: "invitation accepted"})
}

// CreateChildAccount godoc
// @Summary      Create child account within family
// @Description  Creates local account without email requirement (admin/parent only)
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        id path int true "Family ID"
// @Param        body body CreateChildRequest true "Child account details"
// @Success      201 {object} UserResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{id}/children [post]
func (h *FamiliesHandler) CreateChildAccount(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid family id"})
	}

	// Verify caller is admin or parent
	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, callerID).First(&membership).Error; err != nil || (membership.Role != "admin" && membership.Role != "parent") {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only parents or admins can create child accounts"})
	}

	var req CreateChildRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "display_name is required"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "password must be at least 8 characters"})
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to hash password"})
	}

	// Generate synthetic email
	randSuffix := make([]byte, 6)
	_, _ = rand.Read(randSuffix)
	syntheticEmail := fmt.Sprintf("child_%s@family.local", hex.EncodeToString(randSuffix))

	now := time.Now().UTC()
	childUser := models.User{
		Email:        syntheticEmail,
		PasswordHash: string(pwHash),
		DisplayName:  req.DisplayName,
		Timezone:     "UTC",
		Status:       "active",
		AuthMethod:   "local",
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&childUser).Error; err != nil {
			return err
		}

		childMembership := models.FamilyMembership{
			FamilyID: uint(familyID),
			UserID:   childUser.ID,
			Role:     "child",
			Sharing:  "detailed",
			Status:   "active",
			JoinedAt: now,
		}
		return tx.Create(&childMembership).Error
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create child account"})
	}

	return c.Status(fiber.StatusCreated).JSON(toUserResponse(&childUser))
}

// UpdateSharing godoc
// @Summary      Update sharing level
// @Description  Update sharing level (adult controls own; parent/admin can control child)
// @Tags         families
// @Accept       json
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Param        userId path int true "Target User ID"
// @Param        body body UpdateSharingRequest true "Sharing level"
// @Success      200 {object} MessageResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{familyId}/members/{userId}/sharing [patch]
func (h *FamiliesHandler) UpdateSharing(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.Params("userId"), 10, 32)

	var req UpdateSharingRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Sharing = strings.ToLower(strings.TrimSpace(req.Sharing))
	if req.Sharing != "none" && req.Sharing != "summary" && req.Sharing != "detailed" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "sharing must be none, summary, or detailed"})
	}

	var callerMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, callerID).First(&callerMembership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "not a member of this family"})
	}

	var targetMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, targetUserID).First(&targetMembership).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "target member not found"})
	}

	// Authorization check
	if callerID != uint(targetUserID) {
		// Only admin or parent can modify someone else, and only if target is child
		if (callerMembership.Role != "admin" && callerMembership.Role != "parent") || targetMembership.Role != "child" {
			return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{
				Error: "adult members have private control over their own sharing settings",
			})
		}
	}

	targetMembership.Sharing = req.Sharing
	if err := h.DB.Save(&targetMembership).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update sharing"})
	}

	return c.JSON(MessageResponse{Message: "sharing level updated"})
}

// LeaveFamily godoc
// @Summary      Leave a family
// @Description  Member leaves family
// @Tags         families
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Success      200 {object} MessageResponse
// @Router       /families/{familyId}/leave [post]
func (h *FamiliesHandler) LeaveFamily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)

	var membership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "active membership not found"})
	}

	now := time.Now().UTC()
	membership.Status = "left"
	membership.LeftAt = &now

	if err := h.DB.Save(&membership).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to leave family"})
	}

	return c.JSON(MessageResponse{Message: "left family"})
}

// RemoveMember godoc
// @Summary      Remove a member from family
// @Description  Admin removes a member
// @Tags         families
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Param        userId path int true "Target User ID"
// @Success      200 {object} MessageResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{familyId}/members/{userId} [delete]
func (h *FamiliesHandler) RemoveMember(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.Params("userId"), 10, 32)

	var callerMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, callerID).First(&callerMembership).Error; err != nil || callerMembership.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "only admins can remove members"})
	}

	var targetMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, targetUserID).First(&targetMembership).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "member not found"})
	}

	now := time.Now().UTC()
	targetMembership.Status = "removed"
	targetMembership.LeftAt = &now

	if err := h.DB.Save(&targetMembership).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to remove member"})
	}

	return c.JSON(MessageResponse{Message: "member removed"})
}

// Check sharing authorization for viewing member reports
func (h *FamiliesHandler) checkMemberReportAccess(callerID uint, familyID, targetUserID uint, reportDetailLevel string) error {
	if callerID == targetUserID {
		return nil // Users can always view their own data
	}

	var callerMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, callerID).First(&callerMembership).Error; err != nil {
		return errors.New("not a member of this family")
	}

	var targetMembership models.FamilyMembership
	if err := h.DB.Where("family_id = ? AND user_id = ? AND status = 'active'", familyID, targetUserID).First(&targetMembership).Error; err != nil {
		return errors.New("target user not found in this family")
	}
	if callerMembership.Role != "admin" && callerMembership.Role != "parent" {
		var settings models.FamilySettings
		if err := h.DB.Where("family_id = ?", familyID).First(&settings).Error; err != nil || !settings.MembersCanViewOthers {
			return errors.New("family settings do not allow viewing other members' data")
		}
	}


	// Child member: parents and admins have access
	if targetMembership.Role == "child" {
		if callerMembership.Role == "parent" || callerMembership.Role == "admin" {
			return nil
		}
	}

	// Admin/parent can view any member's data regardless of sharing setting
	if callerMembership.Role == "parent" || callerMembership.Role == "admin" {
		return nil
	}

	// Adult member: check sharing level
	if targetMembership.Sharing == "none" {
		return errors.New("sharing is disabled by this member")
	}

	if targetMembership.Sharing == "summary" {
		if reportDetailLevel == "detailed" {
			return errors.New("member only shares summary reports")
		}
		return nil
	}

	if targetMembership.Sharing == "detailed" {
		return nil
	}

	return errors.New("access denied")
}

// GetMemberScreenTimeDaily godoc
// @Summary      Get family member daily screen time
// @Description  View member screen time with sharing authorization check
// @Tags         families
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Param        userId path int true "Target User ID"
// @Param        from query string false "From date"
// @Param        to query string false "To date"
// @Success      200 {object} ScreenTimeDailyResponse
// @Failure      403 {object} ErrorResponse
// @Router       /families/{familyId}/members/{userId}/reports/screen-time/daily [get]
func (h *FamiliesHandler) GetMemberScreenTimeDaily(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.Params("userId"), 10, 32)

	if err := h.checkMemberReportAccess(callerID, uint(familyID), uint(targetUserID), "summary"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: err.Error()})
	}

	// Temporarily override context userID to targetUserID and call handler
	c.Locals("userID", uint(targetUserID))
	return h.ReportsHandler.GetScreenTimeDaily(c)
}

// GetMemberScreenTimeApps godoc
// @Summary      Get family member app usage
// @Description  View member app usage with sharing authorization check
// @Tags         families
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Param        userId path int true "Target User ID"
// @Param        from query string false "From date"
// @Param        to query string false "To date"
// @Success      200 {array} AppUsageSummary
// @Failure      403 {object} ErrorResponse
// @Router       /families/{familyId}/members/{userId}/reports/screen-time/apps [get]
func (h *FamiliesHandler) GetMemberScreenTimeApps(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.Params("userId"), 10, 32)

	if err := h.checkMemberReportAccess(callerID, uint(familyID), uint(targetUserID), "detailed"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: err.Error()})
	}

	c.Locals("userID", uint(targetUserID))
	return h.ReportsHandler.GetScreenTimeApps(c)
}

// GetMemberHealthDaily godoc
// @Summary      Get family member health totals
// @Description  View member health totals with sharing authorization check
// @Tags         families
// @Produce      json
// @Param        familyId path int true "Family ID"
// @Param        userId path int true "Target User ID"
// @Param        from query string false "From date"
// @Param        to query string false "To date"
// @Success      200 {array} HealthDailySummary
// @Failure      403 {object} ErrorResponse
// @Router       /families/{familyId}/members/{userId}/reports/health/daily [get]
func (h *FamiliesHandler) GetMemberHealthDaily(c fiber.Ctx) error {
	callerID := c.Locals("userID").(uint)
	familyID, _ := strconv.ParseUint(c.Params("familyId"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.Params("userId"), 10, 32)

	if err := h.checkMemberReportAccess(callerID, uint(familyID), uint(targetUserID), "summary"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: err.Error()})
	}

	c.Locals("userID", uint(targetUserID))
	return h.ReportsHandler.GetHealthDailyTotals(c)
}
