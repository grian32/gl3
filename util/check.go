package util

func IsDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func IsAlpha(char byte) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		char == '_'
}

func IsAlphaNumeric(char byte) bool {
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		char == '_'
}

func IsHexaNumeral(char byte) bool {
	return (char >= 'A' && char <= 'F') ||
		(char >= 'a' && char <= 'f') ||
		(char >= '0' && char <= '9')
}
