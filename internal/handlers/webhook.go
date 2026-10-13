package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/ingestion"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	DB           *database.DB
	MaxBodyBytes int64
	ServerTZ     *time.Location
	Bus          *IngestionBus
}

func NewWebhookHandler(db *database.DB, maxBodyBytes int64, tz string, bus *IngestionBus) *WebhookHandler {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return &WebhookHandler{
		DB:           db,
		MaxBodyBytes: maxBodyBytes,
		ServerTZ:     loc,
		Bus:          bus,
	}
}

// HandleWebhook godoc
// @Summary      Ingest webhook payload from device
// @Description  Receive and process sync payload from Life Dashboard Companion
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Param        deviceToken path string true "Device token"
// @Param        X-Signature header string false "HMAC-SHA256 signature (sha256=<hex>)"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} ErrorResponse "Invalid payload"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      404 {object} ErrorResponse "Device not found"
// @Failure      411 {object} ErrorResponse "Length required"
// @Failure      413 {object} ErrorResponse "Payload too large"
// @Router       /webhooks/v1/devices/{deviceToken} [post]
func (h *WebhookHandler) HandleWebhook(c fiber.Ctx) error {
	startTime := time.Now()
	deviceToken := c.Params("deviceToken")
	if deviceToken == "" {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device token required"})
	}

	// 1. Content-Length validation
	clHeader := string(c.Request().Header.Peek("Content-Length"))
	if clHeader == "" {
		return c.Status(fiber.StatusLengthRequired).JSON(ErrorResponse{Error: "Content-Length header required"})
	}
	contentLength, err := strconv.ParseInt(clHeader, 10, 64)
	if err != nil || contentLength <= 0 {
		return c.Status(fiber.StatusLengthRequired).JSON(ErrorResponse{Error: "valid Content-Length required"})
	}
	if contentLength > h.MaxBodyBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(ErrorResponse{Error: "payload exceeds size limit"})
	}

	body := c.Body()
	if int64(len(body)) > h.MaxBodyBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(ErrorResponse{Error: "payload exceeds size limit"})
	}

	// 2. Token lookup
	tokenHash := HashToken(deviceToken)
	var device models.Device
	if err := h.DB.Where("token_hash = ?", tokenHash).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device not found"})
		}
		slog.Error("Database error finding device", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "internal error"})
	}

	if device.Status == "revoked" {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "device has been revoked"})
	}

	// 3. HMAC-SHA256 verification
	if device.WebhookSecret != "" {
		sigHeader := string(c.Request().Header.Peek("X-Signature"))
		if sigHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "missing X-Signature header"})
		}
		expectedSig := computeHMAC(body, []byte(device.WebhookSecret))
		rawSig := strings.TrimPrefix(sigHeader, "sha256=")
		sigBytes, err := hex.DecodeString(rawSig)
		if err != nil || !hmac.Equal(expectedSig, sigBytes) {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "invalid HMAC signature"})
		}
	}

	// 4. Parse payload
	parsed, err := ingestion.ParsePayload(device.ID, body, h.ServerTZ)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid JSON payload: " + err.Error()})
	}

	now := time.Now().UTC()

	// 5. Test ping
	if parsed.IsPing {
		device.LastSyncAt = &now
		if parsed.Source != "" {
			device.LastSyncSource = parsed.Source
		}
		_ = h.DB.Save(&device).Error
		if h.Bus != nil {
			dName := device.DisplayName
			if dName == "" {
				dName = device.Label
			}
			h.Bus.Publish(IngestionEvent{
				Timestamp:  now,
				DeviceID:   device.ID,
				DeviceName: dName,
				Source:     parsed.Source,
				SHA256:     "ping",
				Records:    0,
				Tombstones: 0,
				LatencyMs:  time.Since(startTime).Milliseconds(),
				Status:     "PING_OK",
			})
		}
		return c.JSON(fiber.Map{
			"stored": 0,
			"ping":   true,
		})
	}

	// 6. Duplicate check via body SHA-256
	hSHA := sha256.New()
	hSHA.Write(body)
	bodySHA256 := hex.EncodeToString(hSHA.Sum(nil))

	var existingPayload models.Payload
	if err := h.DB.Where("body_sha256 = ?", bodySHA256).First(&existingPayload).Error; err == nil {
		// Found duplicate
		return c.JSON(fiber.Map{
			"duplicate": true,
			"stored":    0,
		})
	}

	// 7. Transaction
	var storedCount int64
	var skippedCount int64

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		// Save payload
		payloadRecord := &models.Payload{
			DeviceID:    device.ID,
			Source:      parsed.Source,
			Sequence:    parsed.Sequence,
			AppVersion:  parsed.AppVersion,
			BuiltAt:     parsed.BuiltAt,
			ReceivedAt:  now,
			Kind:        parsed.Kind,
			RecordCount: len(parsed.Records),
			BodySHA256:  bodySHA256,
			Body:        string(body),
		}
		if err := tx.Create(payloadRecord).Error; err != nil {
			return err
		}

		// Process deleted records
		for _, del := range parsed.DeletedRecords {
			_, _ = database.DeleteRecords(tx, h.DB.Dialect, device.ID, del.Type, del.UUID, parsed.Sequence, parsed.BuiltAt)
			tomb := &models.Tombstone{
				Type:      del.Type,
				UUID:      del.UUID,
				DeviceID:  device.ID,
				Seq:       parsed.Sequence,
				BuiltAt:   parsed.BuiltAt,
				DeletedAt: now,
			}
			_, _ = database.UpsertTombstone(tx, h.DB.Dialect, tomb)
		}

		// Process records
		for _, rec := range parsed.Records {
			affected, err := database.UpsertRecord(tx, h.DB.Dialect, rec)
			if err != nil {
				return err
			}
			if affected > 0 {
				storedCount++
			} else {
				skippedCount++
			}
		}

		// Process buckets
		for _, b := range parsed.Buckets {
			if err := database.UpsertBucket(tx, b); err != nil {
				return err
			}
		}

		// Update device status
		device.LastSyncAt = &now
		if parsed.Source != "" {
			device.LastSyncSource = parsed.Source
		}
		device.LastSyncError = ""
		if parsed.DeviceModel != "" {
			device.DeviceModel = parsed.DeviceModel
		}
		if parsed.AppVersion != "" {
			device.CollectorVersion = parsed.AppVersion
		}
		return tx.Save(&device).Error
	})
	if err != nil {
		slog.Error("Failed to store webhook payload in transaction", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to process payload"})
	}

	if h.Bus != nil {
		dName := device.DisplayName
		if dName == "" {
			dName = device.Label
		}
		h.Bus.Publish(IngestionEvent{
			Timestamp:  now,
			DeviceID:   device.ID,
			DeviceName: dName,
			Source:     parsed.Source,
			SHA256:     bodySHA256,
			Records:    storedCount,
			Tombstones: int64(len(parsed.DeletedRecords)),
			LatencyMs:  time.Since(startTime).Milliseconds(),
			Status:     "PAYLOAD_OK",
		})
	}
	return c.JSON(fiber.Map{
		"stored":  storedCount,
		"buckets": len(parsed.Buckets),
		"deleted": len(parsed.DeletedRecords),
		"skipped": skippedCount,
	})
}

func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func computeHMAC(message, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	return mac.Sum(nil)
}
