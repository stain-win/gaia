package secutil

// WipeBytes securely zeros a byte slice to remove sensitive data from memory.
// This function should be called (typically deferred) after reading sensitive data
// like passphrases, passwords, or encryption keys to ensure they don't remain
// in memory longer than necessary.
//
// Example usage:
//
//	passphrase, err := term.ReadPassword(int(syscall.Stdin))
//	if err != nil {
//	    return err
//	}
//	defer secutil.WipeBytes(passphrase)
func WipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// WipeString securely wipes a string by converting it to a byte slice and zeroing it.
// Note: This doesn't guarantee the original string is cleared from memory since strings
// are immutable in Go, but it helps clear copies and is better than nothing.
// For maximum security, prefer working with []byte directly when handling sensitive data.
//
// Example usage:
//
//	password := "secret123"
//	defer secutil.WipeString(&password)
func WipeString(s *string) {
	if s == nil || *s == "" {
		return
	}
	// Convert to byte slice and wipe
	b := []byte(*s)
	WipeBytes(b)
	// Clear the original string reference
	*s = ""
}
