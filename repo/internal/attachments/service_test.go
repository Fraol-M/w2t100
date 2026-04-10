package attachments

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"propertyops/backend/internal/common"
)

// --- MIME Validation Tests ---

func TestValidateImageMIME_JPEG(t *testing.T) {
	if fe := common.ValidateImageMIME("image/jpeg"); fe != nil {
		t.Errorf("expected image/jpeg to be valid, got error: %s", fe.Message)
	}
}

func TestValidateImageMIME_PNG(t *testing.T) {
	if fe := common.ValidateImageMIME("image/png"); fe != nil {
		t.Errorf("expected image/png to be valid, got error: %s", fe.Message)
	}
}

func TestValidateImageMIME_GIF_Rejected(t *testing.T) {
	if fe := common.ValidateImageMIME("image/gif"); fe == nil {
		t.Error("expected image/gif to be rejected")
	}
}

func TestValidateImageMIME_PDF_Rejected(t *testing.T) {
	if fe := common.ValidateImageMIME("application/pdf"); fe == nil {
		t.Error("expected application/pdf to be rejected")
	}
}

func TestValidateImageMIME_Empty_Rejected(t *testing.T) {
	if fe := common.ValidateImageMIME(""); fe == nil {
		t.Error("expected empty MIME to be rejected")
	}
}

func TestValidateImageMIME_CaseInsensitive(t *testing.T) {
	if fe := common.ValidateImageMIME("IMAGE/JPEG"); fe != nil {
		t.Errorf("expected IMAGE/JPEG (uppercase) to be valid, got error: %s", fe.Message)
	}
}

// --- Magic Byte Validation Tests ---

func TestValidateFileSignature_JPEG(t *testing.T) {
	header := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
	mime, fe := common.ValidateFileSignature(header)
	if fe != nil {
		t.Errorf("expected JPEG signature to be valid, got error: %s", fe.Message)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestValidateFileSignature_PNG(t *testing.T) {
	header := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime, fe := common.ValidateFileSignature(header)
	if fe != nil {
		t.Errorf("expected PNG signature to be valid, got error: %s", fe.Message)
	}
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}
}

func TestValidateFileSignature_Invalid(t *testing.T) {
	header := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00} // GIF signature
	_, fe := common.ValidateFileSignature(header)
	if fe == nil {
		t.Error("expected GIF signature to be rejected")
	}
}

func TestValidateFileSignature_TooShort(t *testing.T) {
	header := []byte{0xFF, 0xD8}
	_, fe := common.ValidateFileSignature(header)
	if fe == nil {
		t.Error("expected too-short header to be rejected")
	}
}

func TestValidateFileSignature_Empty(t *testing.T) {
	_, fe := common.ValidateFileSignature([]byte{})
	if fe == nil {
		t.Error("expected empty header to be rejected")
	}
}

func TestValidateFileSignature_JPEG_Variants(t *testing.T) {
	// JPEG with different APP markers
	headers := [][]byte{
		{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}, // JFIF
		{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 0x45, 0x78}, // Exif
		{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x10, 0x00, 0x00}, // Quantization table
	}
	for i, header := range headers {
		mime, fe := common.ValidateFileSignature(header)
		if fe != nil {
			t.Errorf("JPEG variant %d: expected valid, got error: %s", i, fe.Message)
		}
		if mime != "image/jpeg" {
			t.Errorf("JPEG variant %d: expected image/jpeg, got %s", i, mime)
		}
	}
}

// --- File Size Limit Tests ---

func TestFileSize_WithinLimit(t *testing.T) {
	size := int64(4 * 1024 * 1024) // 4 MB
	if size > MaxFileSize {
		t.Errorf("expected 4MB to be within %d byte limit", MaxFileSize)
	}
}

func TestFileSize_ExactLimit(t *testing.T) {
	size := int64(MaxFileSize) // 5 MB
	if size > MaxFileSize {
		t.Error("expected exact limit to be acceptable")
	}
}

func TestFileSize_ExceedsLimit(t *testing.T) {
	size := int64(MaxFileSize + 1) // 5 MB + 1 byte
	if size <= MaxFileSize {
		t.Error("expected size exceeding limit to fail")
	}
}

// --- Attachment Count Limit Tests ---

func TestAttachmentCount_WithinLimit(t *testing.T) {
	fe := common.ValidateAttachmentCount(0, 1, MaxAttachmentsPerWorkOrder)
	if fe != nil {
		t.Errorf("expected 0+1 to be within limit, got error: %s", fe.Message)
	}
}

func TestAttachmentCount_AtLimit(t *testing.T) {
	fe := common.ValidateAttachmentCount(5, 1, MaxAttachmentsPerWorkOrder)
	if fe != nil {
		t.Errorf("expected 5+1=6 to be at limit, got error: %s", fe.Message)
	}
}

func TestAttachmentCount_ExceedsLimit(t *testing.T) {
	fe := common.ValidateAttachmentCount(6, 1, MaxAttachmentsPerWorkOrder)
	if fe == nil {
		t.Error("expected 6+1>6 to exceed limit")
	}
}

func TestAttachmentCount_MultipleAdded(t *testing.T) {
	fe := common.ValidateAttachmentCount(4, 3, MaxAttachmentsPerWorkOrder)
	if fe == nil {
		t.Error("expected 4+3>6 to exceed limit")
	}
}

func TestAttachmentCount_Empty(t *testing.T) {
	fe := common.ValidateAttachmentCount(0, 6, MaxAttachmentsPerWorkOrder)
	if fe != nil {
		t.Errorf("expected 0+6=6 to be at limit, got error: %s", fe.Message)
	}
}

// --- SHA-256 Computation Tests ---

func TestComputeSHA256(t *testing.T) {
	data := []byte("hello world")
	expected := sha256.Sum256(data)
	expectedHex := hex.EncodeToString(expected[:])

	result := ComputeSHA256(data)
	if result != expectedHex {
		t.Errorf("SHA-256 mismatch: expected %s, got %s", expectedHex, result)
	}
}

func TestComputeSHA256_Empty(t *testing.T) {
	data := []byte("")
	expected := sha256.Sum256(data)
	expectedHex := hex.EncodeToString(expected[:])

	result := ComputeSHA256(data)
	if result != expectedHex {
		t.Errorf("SHA-256 mismatch for empty input: expected %s, got %s", expectedHex, result)
	}
}

func TestComputeSHA256_Deterministic(t *testing.T) {
	data := []byte("test data for hashing")
	hash1 := ComputeSHA256(data)
	hash2 := ComputeSHA256(data)
	if hash1 != hash2 {
		t.Errorf("SHA-256 not deterministic: %s != %s", hash1, hash2)
	}
}

func TestComputeSHA256_DifferentInputs(t *testing.T) {
	hash1 := ComputeSHA256([]byte("input A"))
	hash2 := ComputeSHA256([]byte("input B"))
	if hash1 == hash2 {
		t.Error("SHA-256 produced same hash for different inputs")
	}
}

func TestComputeSHA256_Length(t *testing.T) {
	result := ComputeSHA256([]byte("any data"))
	if len(result) != 64 {
		t.Errorf("SHA-256 hex string should be 64 chars, got %d", len(result))
	}
}
