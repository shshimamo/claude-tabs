package main

// TerminalScripts defines AppleScript templates for terminal operations.
type TerminalScripts struct {
	Focus    string `json:"focus"`
	Activate string `json:"activate"`
	Input    string `json:"input"`
	Keys     string `json:"keys"`
	KeyCmd   string `json:"key_cmd"` // template for {{CMDS}} in Keys, use {{KEY}} placeholder
	NewTab   string `json:"new_tab"`
	Screen   string `json:"screen"`
}

var builtinTerminalPresets = map[string]TerminalScripts{
	"iterm2": {
		Focus: `tell application "iTerm2"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					select t
					tell w to select
					return "found"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		Activate: `tell application "iTerm2" to activate`,
		Input: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					tell s to write text "{{TEXT}}"
					return "sent"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		Keys: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					{{CMDS}}
					return "sent"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		KeyCmd: `tell s to write text "{{KEY}}" newline NO
					delay 0.1
					tell s to write text ""`,
		NewTab: `tell application "iTerm2"
	tell current window
		create tab with default profile
		tell current session of current tab
			write text "{{COMMAND}}"
			return tty
		end tell
	end tell
end tell`,
		Screen: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					return contents of s
				end if
			end repeat
		end repeat
	end repeat
	return ""
end tell`,
	},
	"terminal": {
		Focus: `tell application "Terminal"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "{{TTY}}" then
				set selected tab of w to t
				set index of w to 1
				return "found"
			end if
		end repeat
	end repeat
	return "not_found"
end tell`,
		Activate: `tell application "Terminal" to activate`,
		Input: `tell application "Terminal"
	do script "{{TEXT}}" in (first tab of first window whose tty is "{{TTY}}")
	return "sent"
end tell`,
		Keys: `tell application "System Events"
	tell process "Terminal"
		{{CMDS}}
	end tell
end tell
return "sent"`,
		KeyCmd: `keystroke "{{KEY}}"
		delay 0.1
		keystroke return`,
		NewTab: `tell application "Terminal"
	do script "{{COMMAND}}"
	return tty of selected tab of front window
end tell`,
		Screen: `tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "{{TTY}}" then
				return history of t
			end if
		end repeat
	end repeat
	return ""
end tell`,
	},
}

func getTerminalScripts(cfg Config) TerminalScripts {
	terminal := cfg.Terminal
	if terminal == "" {
		terminal = "iterm2"
	}
	if cfg.TerminalPresets != nil {
		if ts, ok := cfg.TerminalPresets[terminal]; ok {
			return ts
		}
	}
	if ts, ok := builtinTerminalPresets[terminal]; ok {
		return ts
	}
	return builtinTerminalPresets["iterm2"]
}
