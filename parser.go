package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	tilde = iota + 1
	hat
)

type specifyVersion interface {
	version() int
}

type (
	bigger      int
	smaller     int
	sameBigger  int
	sameSmaller int
	same        int
	asterisk    struct{}
)

func (i bigger) version() int      { return int(i) }
func (i smaller) version() int     { return int(i) }
func (i sameBigger) version() int  { return int(i) }
func (i sameSmaller) version() int { return int(i) }
func (i same) version() int        { return int(i) }
func (i asterisk) version() int    { return 0 }

type version struct {
	major specifyVersion
	minor specifyVersion
	patch specifyVersion
}

func compareVersion(number int, version specifyVersion) bool {
	switch v := version.(type) {
	case bigger:
		return int(v) < number
	case smaller:
		return int(v) > number
	case sameBigger:
		return int(v) <= number
	case sameSmaller:
		return int(v) >= number
	case same:
		return int(v) == number
	case asterisk:
		return true
	default:
		panic(fmt.Sprintf("unknown type %T", v))
	}
}

func parseVersionString(str string) (*version, error) {
	str = strings.Trim(str, "v/")
	if strings.ContainsAny(str, "<>= ") {
		return nil, fmt.Errorf("not yet supported")
	}
	splits := strings.Split(str, ".")
	if len(splits) == 0 {
		return nil, fmt.Errorf("%s is not support format", str)
	}

	first := splits[0]
	if len(first) == 0 {
		return nil, fmt.Errorf("%s is not support format", str)
	}

	var typ int
	switch first[0] {
	case '~':
		typ = tilde
	case '^':
		typ = hat
	}

	v := &version{
		major: asterisk{},
		minor: asterisk{},
		patch: asterisk{},
	}
	for i, numstr := range splits {
		var (
			num int64
			err error
		)
		if i == 0 && typ > 0 {
			num, err = strconv.ParseInt(numstr[1:], 10, 64)
			if err != nil {
				return nil, err
			}
		} else {
			if numstr == "X" || numstr == "x" {
				continue
			}
			num, err = strconv.ParseInt(numstr, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		switch i {
		case 0:
			v.major = same(num)
		case 1:
			v.minor = same(num)
		case 2:
			if typ == 0 {
				v.patch = same(num)
			}
			if typ == tilde {
				v.patch = sameBigger(num)
			}
			if typ == hat {
				if v.major == same(0) && v.minor == same(0) {
					v.patch = same(num)
				} else {
					v.patch = sameBigger(num)
				}
			}
		}
	}

	return v, nil
}
