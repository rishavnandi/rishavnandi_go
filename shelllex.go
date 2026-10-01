package main

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// chroma's shell lexer only knows shell syntax, so it emits Text for
// `sudo apt update` and every bare command -- shell blocks came out flat while
// yaml and powershell coloured fine. This replaces the bash/sh/shell/zsh lexers
// with one that also marks command position, flags, arguments and numbers.
// The trailing catch-all matters: a RegexLexer emits Error for any input no
// rule matches, which renders as chroma's red error colour.
var shellLexer = chroma.MustNewLexer(
	&chroma.Config{
		Name:    "bash",
		Aliases: []string{"bash", "sh", "shell", "zsh"},
	},
	func() chroma.Rules {
		return chroma.Rules{
			"root": {
				// Order matters: strings and comments first so a # or -- inside
				// them is not mistaken for a comment or a flag.
				{Pattern: `#.*$`, Type: chroma.CommentSingle},
				{Pattern: `"(\\.|[^"\\])*"`, Type: chroma.LiteralStringDouble},
				{Pattern: `'[^']*'`, Type: chroma.LiteralStringSingle},
				{Pattern: `\$\{[^}]*\}|\$[\w@*#?!$]+|\$\d`, Type: chroma.NameVariable},
				{Pattern: `https?://\S+`, Type: chroma.LiteralStringOther},
				// Command position: start of a line, or after a separator.
				{
					Pattern: `(?m)(^|(?:\||&&|\|\||;|\n)\s*)([a-zA-Z_][\w.+-]*)`,
					Type:    chroma.ByGroups(chroma.Text, chroma.NameBuiltin),
				},
				{Pattern: `-{1,2}[a-zA-Z][\w-]*`, Type: chroma.NameTag},
				{Pattern: `[|&;<>]+|=`, Type: chroma.Operator},
				{Pattern: `\b\d+(?:\.\d+)?\b`, Type: chroma.LiteralNumber},
				// Arguments and punctuation, so nothing is left uncoloured.
				{Pattern: `[\w./~@+-]+`, Type: chroma.NameOther},
				{Pattern: `[^\S\n]+|\n`, Type: chroma.Text},
				{Pattern: `.`, Type: chroma.Text},
			},
		}
	},
)

// chroma's yaml lexer returns a single Literal token for a whole
// `ansible_user = <username>` line, so a key and its value share one colour and
// the block reads as plain text. This splits key from value and colours section
// headers, covering both yaml (`key: value`) and ini-style ansible inventory
// (`key = value`, `[section]`).
var yamlLexer = chroma.MustNewLexer(
	&chroma.Config{
		Name:    "yaml",
		Aliases: []string{"yaml", "yml", "ini"},
	},
	func() chroma.Rules {
		return chroma.Rules{
			"root": {
				{Pattern: `#.*$`, Type: chroma.CommentSingle},
				{Pattern: `"(\\.|[^"\\])*"`, Type: chroma.LiteralStringDouble},
				{Pattern: `'[^']*'`, Type: chroma.LiteralStringSingle},
				// Placeholders the posts use for values you fill in.
				{Pattern: `<[^>\n]*>`, Type: chroma.NameTag},
				// `[homeserver:vars]` as a section header.
				{
					Pattern: `(?m)^\s*(\[[^\]]*\])`,
					Type:    chroma.ByGroups(chroma.NameNamespace),
				},
				// `key = value` and `key: value` at the head of a line.
				{
					Pattern: `(?m)^([ \t]*)((?:-\s+)?)([\w.$-]+)([ \t]*[:=][ \t]*)(.*)$`,
					Type: chroma.ByGroups(
						chroma.Text, chroma.Operator, chroma.NameTag,
						chroma.Operator, chroma.LiteralString,
					),
				},
				// A yaml sequence item: `- value`.
				{
					Pattern: `(?m)^([ \t]*)(-)(\s+)(.*)$`,
					Type: chroma.ByGroups(
						chroma.Text, chroma.Operator, chroma.Text, chroma.NameOther,
					),
				},
				// A bare scalar on its own line.
				{
					Pattern: `(?m)^([ \t]*)([-\w.$-]+)[ \t]*$`,
					Type:    chroma.ByGroups(chroma.Text, chroma.NameOther),
				},
				{Pattern: `\b\d+(?:\.\d+)?\b`, Type: chroma.LiteralNumber},
				{Pattern: `[^\S\n]+|\n`, Type: chroma.Text},
				{Pattern: `.`, Type: chroma.Text},
			},
		}
	},
)

func init() {
	lexers.Register(shellLexer)
	lexers.Register(yamlLexer)
}
