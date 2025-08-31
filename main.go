package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	qrcodeGen "github.com/skip2/go-qrcode"
	qrcodeRead "github.com/tuotoo/qrcode"
)

func convertCRC16(input string) string {
	crc := 0xFFFF
	for _, char := range input {
		crc ^= int(char) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = crc << 1
			}
		}
	}
	hexValue := crc & 0xFFFF
	hexString := strings.ToUpper(fmt.Sprintf("%X", hexValue))
	if len(hexString) == 3 {
		hexString = "0" + hexString
	}
	return hexString
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
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
		qrMatrix, err := qrcodeRead.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode QR")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to decode QR"})
		}
		log.Info().Msg("String extracted from QR")
		return c.JSON(fiber.Map{"text": qrMatrix.Content})
	})

	apiV1.Post("/qris-statis-to-dinamis", func(c *fiber.Ctx) error {
		var body struct {
			Amount     string `json:"amount"`
			StaticQRIS string `json:"static_qris"`
		}
		if err := c.BodyParser(&body); err != nil {
			log.Error().Err(err).Msg("Failed to parse body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}
		qris := body.StaticQRIS
		qty := body.Amount
		if len(qris) < 4 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "static_qris too short"})
		}
		qris = qris[:len(qris)-4]
		step1 := strings.Replace(qris, "010211", "010212", 1)
		step2 := strings.Split(step1, "5802ID")
		if len(step2) != 2 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "static_qris format error"})
		}
		uang := fmt.Sprintf("54%02d%s", len(qty), qty)
		uang += "5802ID"
		fix := strings.TrimSpace(step2[0]) + uang + strings.TrimSpace(step2[1])
		fix += convertCRC16(fix)
		png, err := qrcodeGen.Encode(fix, qrcodeGen.Medium, 256)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate QR")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate QR"})
		}
		b64 := base64.StdEncoding.EncodeToString(png)
		log.Info().Msg("QRIS dinamis generated")
		return c.JSON(fiber.Map{"dinamis_qris": fix, "qr_base64": b64})
	})

	log.Info().Msg(fmt.Sprintf("Starting Fiber server on :%s", appPort))
	app.Listen(fmt.Sprintf(":%s", appPort))
}
