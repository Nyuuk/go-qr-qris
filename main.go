package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	goqr "github.com/liyue201/goqr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	qrcodeGen "github.com/skip2/go-qrcode"
)

// computeCRC16CCITT computes CRC-16/CCITT-FALSE (poly 0x1021, init 0xFFFF, xorout 0x0000)
func computeCRC16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if (crc & 0x8000) != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func crcHexUpper4(s string) string {
	// CRC is calculated on bytes of the string
	crc := computeCRC16CCITT([]byte(s))
	return strings.ToUpper(fmt.Sprintf("%04X", crc))
}

type emvTag struct {
	id    string
	value string
}

func parseEMV(s string) ([]emvTag, error) {
	var tags []emvTag
	i := 0
	n := len(s)
	for i < n {
		if i+4 > n {
			return nil, errors.New("malformed EMV data: incomplete tag header")
		}
		id := s[i : i+2]
		lenStr := s[i+2 : i+4]
		l, err := strconv.Atoi(lenStr)
		if err != nil {
			return nil, fmt.Errorf("invalid length for tag %s: %v", id, err)
		}
		if i+4+l > n {
			return nil, fmt.Errorf("malformed EMV data: tag %s length out of range", id)
		}
		val := s[i+4 : i+4+l]
		tags = append(tags, emvTag{id: id, value: val})
		i = i + 4 + l
	}
	return tags, nil
}

func rebuildEMVExcluding(tags []emvTag, excludeIDs map[string]bool) string {
	var b strings.Builder
	for _, t := range tags {
		if excludeIDs[t.id] {
			continue
		}
		b.WriteString(t.id)
		b.WriteString(fmt.Sprintf("%02d", len(t.value)))
		b.WriteString(t.value)
	}
	return b.String()
}

func insertTagBefore(tags []emvTag, beforeID string, newTag emvTag) []emvTag {
	res := make([]emvTag, 0, len(tags)+1)
	inserted := false
	for _, t := range tags {
		if !inserted && t.id == beforeID {
			res = append(res, newTag)
			inserted = true
		}
		res = append(res, t)
	}
	if !inserted {
		// if not found, append at end (before checksum normally)
		res = append(res, newTag)
	}
	return res
}

func formatAmountTag(amountStr string) (string, error) {
	// Expect amountStr to be integer string (e.g. "15000") OR decimal with dot "15000.00"
	// Convert to plain integer string without decimals for QRIS examples used here.
	// If user passes decimal like 12.50 => we will remove dot and preserve cents (not common in IDR).
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return "", errors.New("empty amount")
	}
	// If contains dot, remove dot
	if strings.Contains(amountStr, ".") {
		// validate numeric
		parts := strings.Split(amountStr, ".")
		if len(parts) != 2 {
			return "", errors.New("invalid amount format")
		}
		intPart := parts[0]
		decPart := parts[1]
		// normalize decimal to at most 2 digits
		if len(decPart) > 2 {
			decPart = decPart[:2]
		} else if len(decPart) == 1 {
			decPart = decPart + "0"
		} else if len(decPart) == 0 {
			decPart = "00"
		}
		amt := intPart + decPart
		if _, err := strconv.Atoi(amt); err != nil {
			return "", errors.New("amount contains non-numeric characters")
		}
		return fmt.Sprintf("54%02d%s", len(amt), amt), nil
	}
	// no dot: treat as whole currency (IDR). tag value = digits of amount (e.g. 15000)
	if _, err := strconv.Atoi(amountStr); err != nil {
		return "", errors.New("amount contains non-numeric characters")
	}
	val := amountStr
	return fmt.Sprintf("54%02d%s", len(val), val), nil
}

func getEnv(key string, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg(".env file not found, using default environment variables")
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	appName := getEnv("APP_NAME", "go-qr-qris")
	appPort := getEnv("APP_PORT", "3000")

	app := fiber.New()
	apiV1 := app.Group(fmt.Sprintf("/api/%s/v1", appName)).Name(appName)

	apiV1.Post("/qris-statis-to-dinamis", func(c *fiber.Ctx) error {
		var body struct {
			Amount     string `json:"amount"`
			StaticQRIS string `json:"static_qris"`
		}
		if err := c.BodyParser(&body); err != nil {
			log.Error().Err(err).Msg("Failed to parse body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}

		static := strings.TrimSpace(body.StaticQRIS)
		amount := strings.TrimSpace(body.Amount)

		if len(static) < 10 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "static_qris too short"})
		}

		// find checksum tag "6304" position
		idx := strings.Index(static, "6304")
		if idx == -1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "static_qris missing checksum tag 63"})
		}
		core := static[:idx] // exclude existing 63 and checksum

		// parse EMV tags from core
		tags, err := parseEMV(core)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse EMV tags")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to parse static_qris: " + err.Error()})
		}

		// build new amount tag
		amountTag, err := formatAmountTag(amount)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid amount: " + err.Error()})
		}

		// remove any existing 54 tags (transaction amount) so we won't duplicate
		exclude := map[string]bool{"54": true}

		// we will insert the new 54 before the country code tag "58" if present,
		// otherwise before the tag "62" if present, otherwise append before checksum.
		var rebuiltList []emvTag
		for _, t := range tags {
			if exclude[t.id] {
				continue
			}
			rebuiltList = append(rebuiltList, t)
		}

		// prepare new tag struct for 54
		new54 := emvTag{id: "54", value: amountTag[4:]} // amountTag = "54" + len(2) + value; skip header
		// decide insertion point
		insertBefore := "58" // country code usually 58
		found := false
		for _, t := range rebuiltList {
			if t.id == insertBefore {
				found = true
				break
			}
		}
		if !found {
			insertBefore = "62"
			found = false
			for _, t := range rebuiltList {
				if t.id == insertBefore {
					found = true
					break
				}
			}
		}
		// now insert
		finalTags := insertTagBefore(rebuiltList, insertBefore, new54)

		// rebuild EMV core (without checksum)
		newCore := rebuildEMVExcluding(finalTags, map[string]bool{})

		// compute CRC over newCore + "6304"
		crcInput := newCore + "6304"
		crc := crcHexUpper4(crcInput)
		finalQR := newCore + "6304" + crc

		// generate QR image
		png, err := qrcodeGen.Encode(finalQR, qrcodeGen.Medium, 256)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate QR image")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate QR image"})
		}
		b64 := base64.StdEncoding.EncodeToString(png)

		log.Info().Str("final_qr", finalQR).Msg("QRIS dinamis generated")
		return c.JSON(fiber.Map{"dinamis_qris": finalQR, "qr_base64": b64})
	})

	apiV1.Post("/string-to-qr", func(c *fiber.Ctx) error {
		var body struct {
			Text string `json:"text"`
		}
		if err := c.BodyParser(&body); err != nil {
			log.Error().Err(err).Msg("Failed to parse body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}
		png, err := qrcodeGen.Encode(body.Text, qrcodeGen.Medium, 256)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate QR")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate QR"})
		}
		b64 := base64.StdEncoding.EncodeToString(png)
		log.Info().Msg("QR generated from string")
		return c.JSON(fiber.Map{"qr_base64": b64})
	})

	apiV1.Post("/qr-to-string", func(c *fiber.Ctx) error {
		var body struct {
			QRBase64 string `json:"qr_base64"`
		}
		if err := c.BodyParser(&body); err != nil {
			log.Error().Err(err).Msg("Failed to parse body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}
		imgBytes, err := base64.StdEncoding.DecodeString(body.QRBase64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode base64")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid base64"})
		}
		img, _, err := image.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode image")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid image format"})
		}
		qrCodes, err := goqr.Recognize(img)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode QR (goqr)")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to decode QR"})
		}
		if len(qrCodes) == 0 {
			log.Error().Msg("No QR code found in image")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "No QR code found"})
		}
		log.Info().Msg("String extracted from QR (goqr)")
		return c.JSON(fiber.Map{"text": string(qrCodes[0].Payload)})
	})

	log.Info().Msg(fmt.Sprintf("Starting Fiber server on :%s", appPort))
	app.Listen(fmt.Sprintf(":%s", appPort))
}
