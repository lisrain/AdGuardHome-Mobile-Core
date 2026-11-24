package home

import (
	"fmt"

	"github.com/AdguardTeam/golibs/container"
)

// TODO(a.garipov): Get rid of a global or generate from .twosky.json.
//
// This fork ships only the locales that are bundled with client_v2.
var allowedLanguages = container.NewMapSet(
	"en",
	"zh-cn",
	"zh-tw",
)

// validateLang returns a standard error about if lang is an unknown language.
// If allowEmpty is true, the language can also be empty.
func validateLang(lang string, allowEmpty bool) (err error) {
	if allowEmpty && lang == "" {
		return nil
	}

	if !allowedLanguages.Has(lang) {
		return fmt.Errorf("unknown language: %q", lang)
	}

	return nil
}
