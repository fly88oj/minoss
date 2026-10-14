// Copyright (c) 2015-2021 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package color

import (
	"github.com/fatih/color"
)

// IsTerminal reports whether ANSI coloring is enabled; fatih/color
// disables it automatically on dumb terminals and non-TTY output.
func IsTerminal() bool {
	return !color.NoColor
}

// The returned format/sprint functions check color.NoColor on every
// call, so coloring reacts to TurnOff()/TurnOn() at runtime.
var (
	Bold       = color.New(color.Bold).SprintfFunc()
	RedBold    = color.New(color.FgRed, color.Bold).SprintFunc()
	RedBoldf   = color.New(color.FgRed, color.Bold).SprintfFunc()
	Red        = color.New(color.FgRed).SprintfFunc()
	Blue       = color.New(color.FgBlue).SprintfFunc()
	Yellow     = color.New(color.FgYellow).SprintfFunc()
	Green      = color.New(color.FgGreen).SprintFunc()
	Greenf     = color.New(color.FgGreen).SprintfFunc()
	GreenBold  = color.New(color.FgGreen, color.Bold).SprintFunc()
	CyanBold   = color.New(color.FgCyan, color.Bold).SprintFunc()
	YellowBold = color.New(color.FgYellow, color.Bold).SprintfFunc()
	BlueBold   = color.New(color.FgBlue, color.Bold).SprintfFunc()
	BgYellow   = color.New(color.BgYellow).SprintfFunc()
	Black      = color.New(color.FgBlack).SprintfFunc()
	FgRed      = color.New(color.FgRed).SprintFunc()
	BgRed      = color.New(color.BgRed).SprintfFunc()
	FgWhite    = color.New(color.FgWhite).SprintfFunc()
)

// TurnOff disables ANSI coloring globally.
func TurnOff() {
	color.NoColor = true
}

// TurnOn enables ANSI coloring globally.
func TurnOn() {
	color.NoColor = false
}
