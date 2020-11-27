/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package utils

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/statiko-dev/statiko/shared/utils"
)

// ManifestRuleOptions is used by the AppManifest struct to represent options for a specific location or file type
type ManifestRuleOptions struct {
	Deny          bool              `yaml:"deny"`
	ClientCaching string            `yaml:"clientCaching"`
	Headers       map[string]string `yaml:"headers"`
	CleanHeaders  map[string]string `yaml:"-"`
	Proxy         string            `yaml:"proxy"`
}

// ManifestRule is the dictionary with rules
type ManifestRule struct {
	// An "exact" match equals to a = modifier in the nginx location block
	Exact string `yaml:"exact"`
	// A "prefix" match equals to a ^~ modifier in the nginx location block
	// Exception is the prefix "/", which doesn't use any modifier
	Prefix string `yaml:"prefix"`
	// A "match" match equals to ~* modifier (or ~ modifier if "caseSensitive" is true) in the nginx location block
	Match         string `yaml:"match"`
	CaseSensitive bool   `yaml:"caseSensitive"`
	// Using "file" lets user list file extensions or extension groups (such as "_images")
	File string `yaml:"file"`

	// Options to apply
	Options ManifestRuleOptions `yaml:"options"`
}

// ManifestRules is a slice of ManifestRule structs
type ManifestRules []ManifestRule

// AppManifest represents the manifest of an app
type AppManifest struct {
	Rules   ManifestRules     `yaml:"rules"`
	Rewrite map[string]string `yaml:"rewrite"`
	Page403 string            `yaml:"page403"`
	Page404 string            `yaml:"page404"`

	// Internal
	Locations map[string]ManifestRuleOptions `yaml:"-"`
}

// Sanitize the app's manifest
func (manifest *AppManifest) Sanitize() {
	if manifest == nil {
		return
	}

	// Parse and validate the app's manifest
	manifest.Locations = make(map[string]ManifestRuleOptions)
	if manifest.Rules != nil && len(manifest.Rules) > 0 {
		for _, v := range manifest.Rules {
			// Ensure that only one of the various match types (exact, prefix, match, file) is set
			if (v.Match != "" && (v.Exact != "" || v.File != "" || v.Prefix != "")) ||
				(v.Exact != "" && (v.Match != "" || v.File != "" || v.Prefix != "")) ||
				(v.File != "" && (v.Match != "" || v.Exact != "" || v.Prefix != "")) ||
				(v.Prefix != "" && (v.Match != "" || v.Exact != "" || v.File != "")) {
				logger.Println("Ignoring rule that has more than one match type")
				continue
			}

			// Get the location rule block
			location := ""
			if v.Exact != "" {
				location = "= " + v.Exact
			} else if v.Prefix != "" {
				if v.Prefix == "/" {
					location = "/"
				} else {
					location = "^~ " + v.Prefix
				}
			} else if v.Match != "" {
				if v.CaseSensitive {
					location = "~ " + v.Match
				} else {
					location = "~* " + v.Match
				}
			} else if v.File != "" {
				switch v.File {
				// Aliases
				case "_images":
					location = " ~* \\.(jpg|jpeg|png|gif|ico|svg|svgz|webp|tif|tiff|dng|psd|heif|bmp)$"
				case "_videos":
					location = "~* \\.(mp4|m4v|mkv|webm|avi|mpg|mpeg|ogg|wmv|flv|mov)$"
				case "_audios":
					location = "~* \\.(mp3|mp4|aac|m4a|flac|wav|ogg|wma)$"
				case "_fonts":
					location = "~* \\.(woff|woff2|eot|otf|ttf)$"
				default:
					// Replace all commas with |
					v.File = strings.ReplaceAll(v.File, ",", "|")
					// TODO: validate this with a regular expression
					location = "~* \\.(" + v.File + ")$"
				}
			} else {
				logger.Println("Ignoring rule that has no match type")
				continue
			}

			// Sanitize rule options
			options := sanitizeManifestRuleOptions(v.Options)

			// Add the element
			manifest.Locations[location] = options
		}
	}

	// Ensure that Page404 and Page403 don't start with a /
	if len(manifest.Page404) > 1 && manifest.Page404[0] == '/' {
		manifest.Page404 = manifest.Page404[1:]
	}
	if len(manifest.Page403) > 1 && manifest.Page403[0] == '/' {
		manifest.Page403 = manifest.Page403[1:]
	}
}

// Compile the regular expression for matching ClientCaching values in apps' manifests
var clientCachingRegexp = regexp.MustCompile(`^[1-9][0-9]*(ms|s|m|h|d|w|M|y)$`)

// Validates and sanitizes an ManifestRuleOptions object in the manifest
func sanitizeManifestRuleOptions(v ManifestRuleOptions) ManifestRuleOptions {
	// If there's a ClientCaching value, ensure it's valid
	if v.ClientCaching != "" {
		if !clientCachingRegexp.MatchString(v.ClientCaching) {
			logger.Println("Ignoring invalid value for clientCaching:", v.ClientCaching)
			v.ClientCaching = ""
		}
	}

	// Escape values in headers
	if v.Headers != nil && len(v.Headers) > 0 {
		v.CleanHeaders = make(map[string]string, 0)
		for hk, hv := range v.Headers {
			// Filter out disallowed headers
			if !utils.HeaderIsAllowed(hk) {
				logger.Println("Ignoring invalid header:", hk)
				continue
			}
			// Escape the header
			v.CleanHeaders[escapeConfigString(hk)] = escapeConfigString(hv)
		}
	}

	// Validate the URL for proxying
	if v.Proxy != "" {
		parsed, err := url.ParseRequestURI(v.Proxy)
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			logger.Println("Ignoring invalid value for proxy:", v.Proxy)
			v.Proxy = ""
		}
	}

	return v
}

// Escapes characters in strings used in nginx's config files
func escapeConfigString(in string) (out string) {
	out = ""
	for _, char := range in {
		// Escape " and \
		if char == '"' || char == '\\' {
			out += "\\"
		}
		out += string(char)

		// We can't escape the $ sign, but we can use the $dollar variable
		// See: https://serverfault.com/a/854600/93929
		if char == '$' {
			out += "{dollar}"
		}
	}
	return
}
