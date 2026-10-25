package main

import (
	"strings"
)

var (
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
	currentLang                  = "tr"
)

// DetectSystemLanguage returns "tr" if Windows UI language is Turkish (0x1f), otherwise "en".
func DetectSystemLanguage() string {
	if procGetUserDefaultUILanguage.Find() == nil {
		r, _, _ := procGetUserDefaultUILanguage.Call()
		// Primary language ID is lower 10 bits; 0x1F = LANG_TURKISH
		if (r & 0x3FF) == 0x1F {
			return "tr"
		}
		return "en"
	}
	return "tr"
}

func SetLanguage(lang string) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "en", "eng", "english", "1":
		currentLang = "en"
	case "tr", "tur", "turkish", "türkçe", "turkce", "2":
		currentLang = "tr"
	default:
		currentLang = DetectSystemLanguage()
	}
}

func IsTR() bool {
	return currentLang == "tr"
}

// T returns the localized string based on currentLang ("tr" or "en").
func T(tr, en string) string {
	if currentLang == "tr" {
		return tr
	}
	return en
}
