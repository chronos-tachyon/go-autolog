package autolog

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var gPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 64))
	},
}

type parseState uint

const (
	initState parseState = iota
	percentState
	widthState
	dotState
	precState
)

var pstateNames = [...]string{
	"initState",
	"percentState",
	"widthState",
	"dotState",
	"precState",
}

func (ps parseState) GoString() string {
	return pstateNames[ps]
}

func (ps parseState) String() string {
	return ps.GoString()
}

var (
	_ fmt.GoStringer = parseState(0)
	_ fmt.Stringer   = parseState(0)
)

type formatState struct {
	Width       uint
	Prec        uint
	Pad         rune
	HasWidth    bool
	HasPrec     bool
	JustifyLeft bool
}

func (fs *formatState) Reset() {
	*fs = formatState{}
}

func (fs *formatState) SetDefaultPad(pad rune) {
	if fs.Pad != 0 {
		return
	}
	fs.Pad = pad
}

func (fs *formatState) SetDefaultWidth(width uint) {
	if fs.HasWidth {
		return
	}
	fs.Width = width
	fs.HasWidth = true
}

func (fs *formatState) SetDefaultPrec(prec uint) {
	if fs.HasPrec {
		return
	}
	fs.Prec = prec
	fs.HasPrec = true
}

func (fs formatState) FormatString(buf *bytes.Buffer, value string) {
	fs.SetDefaultPad(' ')
	if fs.Pad == '+' {
		fs.Pad = '0'
	}
	if fs.JustifyLeft && fs.Pad == '0' {
		fs.Pad = ' '
	}

	if fs.HasPrec {
		p := fs.Prec
		if uint(len(value)) > p {
			value = value[:p]
		}
	}

	if fs.HasWidth && !fs.JustifyLeft {
		n := uint(len(value))
		for n < fs.Width {
			buf.WriteRune(fs.Pad)
			n++
		}
	}

	buf.WriteString(value)

	if fs.HasWidth && fs.JustifyLeft {
		n := uint(len(value))
		for n < fs.Width {
			buf.WriteRune(fs.Pad)
			n++
		}
	}
}

func (fs formatState) FormatInt(buf *bytes.Buffer, value int64) {
	neg := false
	u64 := uint64(value)
	if value < 0 {
		neg = true
		u64 = uint64(-value)
	}
	fs.formatIntInternal(buf, neg, u64)
}

func (fs formatState) FormatUint(buf *bytes.Buffer, value uint64) {
	fs.formatIntInternal(buf, false, value)
}

func (fs formatState) formatIntInternal(buf *bytes.Buffer, neg bool, value uint64) {
	fs.SetDefaultPad('0')

	sign := ""
	if fs.Pad == '+' {
		sign = "+"
		fs.Pad = '0'
	}
	if fs.Pad == '0' && fs.JustifyLeft {
		fs.Pad = ' '
	}
	if neg {
		sign = "-"
	}

	str := strconv.FormatUint(value, 10)

	buf.WriteString(sign)

	if fs.HasWidth && !fs.JustifyLeft {
		n := uint(len(str)) + uint(len(sign))
		for n < fs.Width {
			buf.WriteRune(fs.Pad)
			n++
		}
	}

	buf.WriteString(str)

	if fs.HasWidth && fs.JustifyLeft {
		n := uint(len(str)) + uint(len(sign))
		for n < fs.Width {
			buf.WriteRune(fs.Pad)
			n++
		}
	}
}

func Strftime(pattern string, t time.Time) string {
	buf := gPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		gPool.Put(buf)
	}()

	var ps parseState = initState
	var fs formatState
	fs.Reset()

	fail := func(ch rune) {
		buf.WriteString(fmt.Sprintf("%%!ERR[%v, %v, %q]", ps, fs, ch))
	}

	for _, ch := range pattern {
		switch {
		case ps == initState && ch == '%':
			ps = percentState
		case ps == initState:
			buf.WriteRune(ch)

		case ps == percentState && ch == '0':
			fs.Pad = '0'
		case ps == percentState && ch == '+':
			fs.Pad = '+'
		case ps == percentState && ch == '_':
			fs.Pad = '_'
		case ps == percentState && (ch == '-' || ch == '<'):
			fs.JustifyLeft = true
		case ps == percentState && ch == '>':
			fs.JustifyLeft = false
		case ps == percentState && ch >= '1' && ch <= '9':
			fs.Width = uint(ch - '0')
			fs.HasWidth = true
			ps = widthState
		case ps == percentState && ch == '.':
			ps = dotState

		case ps == widthState && ch >= '0' && ch <= '9':
			fs.Width = fs.Width*10 + uint(ch-'0')
		case ps == widthState && ch == '.':
			ps = dotState

		case ps == dotState && ch >= '0' && ch <= '9':
			fs.Prec = uint(ch - '0')
			fs.HasPrec = true
			ps = precState
		case ps == dotState:
			fail(ch)
			ps = initState

		case ps == precState && ch >= '0' && ch <= '9':
			fs.Prec = fs.Prec*10 + uint(ch-'0')

		case ch == 'A':
			fs.FormatString(buf, t.Format("Monday"))
			fs.Reset()
			ps = initState

		case ch == 'B':
			fs.FormatString(buf, t.Format("January"))
			fs.Reset()
			ps = initState

		case ch == 'C':
			x := parseUint(t.Format("2006"))
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, x/100)
			fs.Reset()
			ps = initState

		case ch == 'D':
			fs.FormatString(buf, t.Format("01/02/06"))
			fs.Reset()
			ps = initState

		// 'E': era modifier

		case ch == 'F':
			fs.FormatString(buf, t.Format("2006-01-02"))
			fs.Reset()
			ps = initState

		// 'G': ISO year-of-week

		case ch == 'H':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("15")))
			fs.Reset()
			ps = initState

		case ch == 'I':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("03")))
			fs.Reset()
			ps = initState

		case ch == 'M':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("04")))
			fs.Reset()
			ps = initState

		// 'O': alternative digit modifier

		case ch == 'P':
			fs.FormatString(buf, t.Format("pm"))
			fs.Reset()
			ps = initState

		case ch == 'R':
			fs.FormatString(buf, t.Format("15:04"))
			fs.Reset()
			ps = initState

		case ch == 'S':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("05")))
			fs.Reset()
			ps = initState

		case ch == 'T':
			fs.FormatString(buf, t.Format("15:04:05"))
			fs.Reset()
			ps = initState

		// 'U': week number, 00-53, 1st Sun is week 01

		// 'V': ISO week number

		// 'W': week number, 00-53, 1st Mon is week 01

		case ch == 'X':
			fs.FormatString(buf, t.Format("15:04:05"))
			fs.Reset()
			ps = initState

		case ch == 'Y':
			fs.SetDefaultWidth(4)
			fs.FormatUint(buf, parseUint(t.Format("2006")))
			fs.Reset()
			ps = initState

		case ch == 'Z':
			fs.FormatString(buf, t.Format("MST"))
			fs.Reset()
			ps = initState

		case ch == 'a':
			fs.FormatString(buf, t.Format("Mon"))
			fs.Reset()
			ps = initState

		case ch == 'b':
			fs.FormatString(buf, t.Format("Jan"))
			fs.Reset()
			ps = initState

		case ch == 'c':
			fs.FormatString(buf, t.Format("Mon Jan _2 15:04:05 2006"))
			fs.Reset()
			ps = initState

		case ch == 'd':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("02")))
			fs.Reset()
			ps = initState

		case ch == 'e':
			fs.SetDefaultPad(' ')
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("02")))
			fs.Reset()
			ps = initState

		// 'g': ISO week-based year, 2 digits

		case ch == 'h':
			fs.FormatString(buf, t.Format("Jan"))
			fs.Reset()
			ps = initState

		// 'j': Julian day of year

		case ch == 'k':
			fs.SetDefaultPad(' ')
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("15")))
			fs.Reset()
			ps = initState

		case ch == 'l':
			fs.SetDefaultPad(' ')
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("03")))
			fs.Reset()
			ps = initState

		case ch == 'm':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("01")))
			fs.Reset()
			ps = initState

		case ch == 'n':
			fs.FormatString(buf, "\n")
			fs.Reset()
			ps = initState

		case ch == 'p':
			fs.FormatString(buf, t.Format("PM"))
			fs.Reset()
			ps = initState

		case ch == 'r':
			fs.FormatString(buf, t.Format("03:04:05 PM"))
			fs.Reset()
			ps = initState

		case ch == 's':
			s := t.Unix()
			fs.FormatUint(buf, uint64(s))
			fs.Reset()
			ps = initState

		case ch == 't':
			fs.FormatString(buf, "\t")
			fs.Reset()
			ps = initState

		// 'u': numeric day of week (Mon=1 Sun=7)

		// 'w': numeric day of week (Sun=0 Sat=6)

		case ch == 'x':
			fs.FormatString(buf, t.Format("2006-01-02"))
			fs.Reset()
			ps = initState

		case ch == 'y':
			fs.SetDefaultWidth(2)
			fs.FormatUint(buf, parseUint(t.Format("06")))
			fs.Reset()
			ps = initState

		case ch == 'z':
			fs.SetDefaultWidth(5)
			fs.FormatInt(buf, parseInt(t.Format("-0700")))
			fs.Reset()
			ps = initState

		case ch == '%':
			fs.FormatString(buf, "%")
			fs.Reset()
			ps = initState

		default:
			fail(ch)
			ps = initState
		}
	}
	return buf.String()
}

func parseInt(str string) int64 {
	i64, err := strconv.ParseInt(str, 10, 0)
	if err != nil {
		panic(fmt.Errorf("strconv.ParseInt: %q: %w", str, err))
	}
	return i64
}

func parseUint(str string) uint64 {
	u64, err := strconv.ParseUint(str, 10, 0)
	if err != nil {
		panic(fmt.Errorf("strconv.ParseUint: %q: %w", str, err))
	}
	return u64
}

func trimLeadingZeroes(str string) (rune, string) {
	sign := false
	neg := false
	if len(str) > 1 && str[0] == '+' {
		sign = true
		str = str[1:]
	}
	if !sign && len(str) > 1 && str[0] == '-' {
		sign = true
		neg = true
		str = str[1:]
	}
	for len(str) > 1 && str[0] == '0' {
		str = str[1:]
	}
	switch {
	case neg:
		return '-', str
	case sign:
		return '+', str
	default:
		return 0, str
	}
}
