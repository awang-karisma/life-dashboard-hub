package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"gorm.io/gorm"
)

var labelRegex = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

type DevicesHandler struct {
	DB        *database.DB
	PublicURL string
}

func NewDevicesHandler(db *database.DB, publicURL string) *DevicesHandler {
	return &DevicesHandler{
		DB:        db,
		PublicURL: strings.TrimRight(publicURL, "/"),
	}
}

type CreateDeviceRequest struct {
	Label       string `json:"label"`
	DisplayName string `json:"display_name"`
}

type UpdateDeviceRequest struct {
	DisplayName string `json:"display_name"`
}

type DeviceResponse struct {
	ID               uint       `json:"id"`
	UserID           uint       `json:"user_id"`
	Label            string     `json:"label"`
	DisplayName      string     `json:"display_name"`
	DeviceModel      string     `json:"device_model"`
	CollectorVersion string     `json:"collector_version"`
	Status           string     `json:"status"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
	LastSyncSource   string     `json:"last_sync_source"`
	LastSyncError    string     `json:"last_sync_error"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
}

type DeviceCredentialsResponse struct {
	DeviceResponse
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"`
	PairingQR     string `json:"pairing_qr"`
}

func toDeviceResponse(d *models.Device) DeviceResponse {
	return DeviceResponse{
		ID:               d.ID,
		UserID:           d.UserID,
		Label:            d.Label,
		DisplayName:      d.DisplayName,
		DeviceModel:      d.DeviceModel,
		CollectorVersion: d.CollectorVersion,
		Status:           d.Status,
		LastSyncAt:       d.LastSyncAt,
		LastSyncSource:   d.LastSyncSource,
		LastSyncError:    d.LastSyncError,
		CreatedAt:        d.CreatedAt,
		RevokedAt:        d.RevokedAt,
	}
}

func generateCredentials() (token string, tokenHash string, secret string, err error) {
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return "", "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(tokenBytes)

	h := sha256.New()
	h.Write([]byte(token))
	tokenHash = hex.EncodeToString(h.Sum(nil))

	secretBytes := make([]byte, 32)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	secret = hex.EncodeToString(secretBytes)

	return token, tokenHash, secret, nil
}

// TODO: later implement config for changing this
const defaultInstanceName = "Life Dashboard Hub"

func (h *DevicesHandler) buildPairingQR(webhookURL, secret, name string) string {
	if name == "" {
		name = defaultInstanceName
	}
	encodedName := strings.ReplaceAll(url.QueryEscape(name), "+", "%20")
	return fmt.Sprintf("lifedashboard://pair#v=1&url=%s&secret=%s&name=%s&sources=health_connect,screen_time",
		url.QueryEscape(webhookURL), secret, encodedName)
}

// CreateDevice godoc
// @Summary      Register a new device
// @Description  Create a device registration and return webhook credentials and QR pairing URI
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        body body CreateDeviceRequest true "Device details"
// @Success      201 {object} DeviceCredentialsResponse
// @Failure      400 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse "Label already in use"
// @Router       /devices [post]
func (h *DevicesHandler) CreateDevice(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	var req CreateDeviceRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Label = strings.ToLower(strings.TrimSpace(req.Label))
	if !labelRegex.MatchString(req.Label) {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "label must be 1-32 lowercase alphanumeric characters, dashes, or underscores",
		})
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = req.Label
	}

	// Check unique label per user
	var existing models.Device
	if err := h.DB.Where("user_id = ? AND label = ?", userID, req.Label).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
			Error: "a device with this label already exists for your account",
		})
	}

	token, tokenHash, secret, err := generateCredentials()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to generate credentials"})
	}

	device := models.Device{
		UserID:        userID,
		Label:         req.Label,
		DisplayName:   displayName,
		TokenHash:     tokenHash,
		WebhookSecret: secret,
		Status:        "active",
	}

	if err := h.DB.Create(&device).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to save device"})
	}

	baseURL := h.PublicURL
	if baseURL == "" {
		baseURL = c.BaseURL()
	}
	webhookURL := fmt.Sprintf("%s/webhooks/v1/devices/%s", baseURL, token)
	pairingQR := h.buildPairingQR(webhookURL, secret, defaultInstanceName)

	return c.Status(fiber.StatusCreated).JSON(DeviceCredentialsResponse{
		DeviceResponse: toDeviceResponse(&device),
		WebhookURL:     webhookURL,
		WebhookSecret:  secret,
		PairingQR:      pairingQR,
	})
}

// ListDevices godoc
// @Summary      List all devices for the current user
// @Description  Returns all registered devices and their sync status
// @Tags         devices
// @Produce      json
// @Success      200 {array} DeviceResponse
// @Router       /devices [get]
func (h *DevicesHandler) ListDevices(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	var devices []models.Device
	if err := h.DB.Where("user_id = ?", userID).Order("id ASC").Find(&devices).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to load devices"})
	}

	result := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		result[i] = toDeviceResponse(&d)
	}

	return c.JSON(result)
}

// GetDevice godoc
// @Summary      Get device details
// @Description  Get a specific device by ID
// @Tags         devices
// @Produce      json
// @Param        id path int true "Device ID"
// @Success      200 {object} DeviceResponse
// @Failure      404 {object} ErrorResponse
// @Router       /devices/{id} [get]
func (h *DevicesHandler) GetDevice(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	idParam := c.Params("id")
	deviceID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid device id"})
	}

	var device models.Device
	if err := h.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
	}

	return c.JSON(toDeviceResponse(&device))
}

// UpdateDevice godoc
// @Summary      Update device details
// @Description  Update display name for a device
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        id path int true "Device ID"
// @Param        body body UpdateDeviceRequest true "Updated details"
// @Success      200 {object} DeviceResponse
// @Failure      404 {object} ErrorResponse
// @Router       /devices/{id} [patch]
func (h *DevicesHandler) UpdateDevice(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	idParam := c.Params("id")
	deviceID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid device id"})
	}

	var device models.Device
	if err := h.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
	}

	var req UpdateDeviceRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if strings.TrimSpace(req.DisplayName) != "" {
		device.DisplayName = strings.TrimSpace(req.DisplayName)
	}

	if err := h.DB.Save(&device).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update device"})
	}

	return c.JSON(toDeviceResponse(&device))
}

// RevokeDevice godoc
// @Summary      Revoke device access
// @Description  Marks a device as revoked so its token is no longer accepted for webhooks
// @Tags         devices
// @Produce      json
// @Param        id path int true "Device ID"
// @Success      200 {object} DeviceResponse
// @Failure      404 {object} ErrorResponse
// @Router       /devices/{id}/revoke [post]
func (h *DevicesHandler) RevokeDevice(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	idParam := c.Params("id")
	deviceID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid device id"})
	}

	var device models.Device
	if err := h.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
	}

	now := time.Now().UTC()
	device.Status = "revoked"
	device.RevokedAt = &now

	if err := h.DB.Save(&device).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to revoke device"})
	}

	return c.JSON(toDeviceResponse(&device))
}

// DeleteDevice godoc
// @Summary      Delete a device and associated data
// @Description  Cascades delete across device records, payloads, buckets, and tombstones
// @Tags         devices
// @Produce      json
// @Param        id path int true "Device ID"
// @Success      200 {object} MessageResponse
// @Failure      404 {object} ErrorResponse
// @Router       /devices/{id} [delete]
func (h *DevicesHandler) DeleteDevice(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	idParam := c.Params("id")
	deviceID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid device id"})
	}

	var device models.Device
	if err := h.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", device.ID).Delete(&models.Record{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", device.ID).Delete(&models.Payload{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", device.ID).Delete(&models.Bucket{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", device.ID).Delete(&models.Tombstone{}).Error; err != nil {
			return err
		}
		return tx.Delete(&device).Error
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to delete device"})
	}

	return c.JSON(MessageResponse{Message: "device and associated data deleted"})
}

// RegenerateDeviceCredentials godoc
// @Summary      Regenerate device credentials
// @Description  Invalidates old token/secret and issues new webhook URL and QR pairing link
// @Tags         devices
// @Produce      json
// @Param        id path int true "Device ID"
// @Success      200 {object} DeviceCredentialsResponse
// @Failure      404 {object} ErrorResponse
// @Router       /devices/{id}/regenerate [post]
func (h *DevicesHandler) RegenerateDeviceCredentials(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	idParam := c.Params("id")
	deviceID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid device id"})
	}

	var device models.Device
	if err := h.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
	}

	token, tokenHash, secret, err := generateCredentials()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to generate credentials"})
	}

	device.TokenHash = tokenHash
	device.WebhookSecret = secret
	device.Status = "active"
	device.RevokedAt = nil

	if err := h.DB.Save(&device).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update credentials"})
	}

	baseURL := h.PublicURL
	if baseURL == "" {
		baseURL = c.BaseURL()
	}
	webhookURL := fmt.Sprintf("%s/webhooks/v1/devices/%s", baseURL, token)
	pairingQR := h.buildPairingQR(webhookURL, secret, defaultInstanceName)

	return c.JSON(DeviceCredentialsResponse{
		DeviceResponse: toDeviceResponse(&device),
		WebhookURL:     webhookURL,
		WebhookSecret:  secret,
		PairingQR:      pairingQR,
	})
}
