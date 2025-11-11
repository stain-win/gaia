package secutil

import (
	"testing"
)

// TestWipeBytes verifies that WipeBytes zeros the byte slice.
func TestWipeBytes(t *testing.T) {
	// Create a test byte slice with sensitive data
	testData := []byte("super-secret-passphrase-12345")
	original := make([]byte, len(testData))
	copy(original, testData)

	// Verify original data is not all zeros
	allZeros := true
	for _, b := range testData {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Fatal("Test data should not be all zeros initially")
	}

	// Wipe the data
	WipeBytes(testData)

	// Verify all bytes are now zero
	for i, b := range testData {
		if b != 0 {
			t.Errorf("Byte at index %d was not wiped (got %d, want 0)", i, b)
		}
	}

	// Verify the original is still intact (we made a copy)
	for i, b := range original {
		if b == 0 {
			t.Errorf("Original data was accidentally modified at index %d", i)
		}
	}
}

// TestWipeBytesEmpty verifies that wiping an empty slice doesn't panic.
func TestWipeBytesEmpty(t *testing.T) {
	var empty []byte
	WipeBytes(empty) // Should not panic
}

// TestWipeBytesNil verifies that wiping a nil slice doesn't panic.
func TestWipeBytesNil(t *testing.T) {
	WipeBytes(nil) // Should not panic
}

// TestWipeString verifies that WipeString clears a string.
func TestWipeString(t *testing.T) {
	password := "super-secret-password-123"
	original := password

	// Wipe the password
	WipeString(&password)

	// Verify the string is now empty
	if password != "" {
		t.Errorf("Password was not cleared (got %q, want empty string)", password)
	}

	// Verify we had the original value
	if original == "" {
		t.Fatal("Original password should not be empty")
	}
}

// TestWipeStringEmpty verifies that wiping an empty string doesn't panic.
func TestWipeStringEmpty(t *testing.T) {
	empty := ""
	WipeString(&empty) // Should not panic
	if empty != "" {
		t.Error("Empty string should remain empty")
	}
}

// TestWipeStringNil verifies that wiping a nil string pointer doesn't panic.
func TestWipeStringNil(t *testing.T) {
	WipeString(nil) // Should not panic
}

// BenchmarkWipeBytes benchmarks the performance of wiping a typical passphrase.
func BenchmarkWipeBytes(b *testing.B) {
	data := []byte("super-secret-passphrase-with-typical-length")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WipeBytes(data)
		// Restore data for next iteration
		copy(data, []byte("super-secret-passphrase-with-typical-length"))
	}
}

// BenchmarkWipeBytesLarge benchmarks wiping a larger data structure.
func BenchmarkWipeBytesLarge(b *testing.B) {
	data := make([]byte, 1024*1024) // 1 MB
	for i := range data {
		data[i] = byte(i % 256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WipeBytes(data)
		// Restore data for next iteration
		for j := range data {
			data[j] = byte(j % 256)
		}
	}
}
