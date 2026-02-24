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

// WipeString sets the string pointer to empty.
//
// Deprecated: This function CANNOT securely wipe Go strings from memory.
// Go strings are immutable — []byte(*s) creates a copy, so zeroing it does NOT clear
// the original string's backing memory. The original bytes remain until garbage collected.
//
// For real security, handle sensitive data as []byte throughout your code and use
// WipeBytes to zero the actual buffer:
//
//	passphrase, _ := term.ReadPassword(int(syscall.Stdin)) // returns []byte
//	defer secutil.WipeBytes(passphrase)
func WipeString(s *string) {
	if s == nil || *s == "" {
		return
	}
	*s = ""
}
