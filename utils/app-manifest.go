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

// ManifestRuleOptions is used by the AppManifest struct to represent options for a specific location or file type
type ManifestRuleOptions struct {
	ClientCaching string            `yaml:"clientCaching"`
	Headers       map[string]string `yaml:"headers"`
	CleanHeaders  map[string]string `yaml:"-"`
}

// ManifestRule is the dictionary with rules
type ManifestRule struct {
	// An "exact" match equals to a = modifier in the nginx location block
	Exact string `yaml:"exact"`
	// A "prefix" match equals to no modifier in the nginx location block
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
