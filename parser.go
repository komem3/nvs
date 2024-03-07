package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	emptyMark = iota
	tiledeMark
	hatMark
	biggerMark
	sameBiggerMark
	smallerMark
	sameSmallerMark
	equalMark
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

func compareVersion(number int, version specifyVersion) (match, confirm bool) {
	vint := version.version()
	switch v := version.(type) {
	case bigger:
		return vint < number, vint < number
	case smaller:
		return vint > number, vint > number
	case sameBigger:
		if vint == number {
			return true, false
		}
		return vint < number, vint < number
	case sameSmaller:
		if vint == number {
			return true, false
		}
		return vint > number, vint > number
	case same:
		return vint == number, false
	case asterisk:
		return true, false
	default:
		panic(fmt.Sprintf("unknown type %T", v))
	}
}

func parseVersionString(str string) (*version, error) {
	str = strings.Trim(str, "v/")
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
		typ = tiledeMark
	case '^':
		typ = hatMark
	case '<':
		if strings.HasPrefix(first, "<=") {
			typ = sameSmallerMark
		} else {
			typ = smallerMark
		}
	case '>':
		if strings.HasPrefix(first, ">=") {
			typ = sameBiggerMark
		} else {
			typ = biggerMark
		}
	}

	v := &version{
		major: asterisk{},
		minor: asterisk{},
		patch: asterisk{},
	}
	for i, numstr := range splits {
		if numstr == "X" || numstr == "x" {
			continue
		}
		num, err := strconv.ParseInt(strings.TrimLeft(numstr, "^~<>="), 10, 64)
		if err != nil {
			return nil, err
		}
		switch i {
		case 0:
			switch typ {
			case biggerMark:
				if len(splits) == 1 {
					v.major = bigger(num)
				} else {
					v.major = sameBigger(num)
				}
			case smallerMark:
				if len(splits) == 1 {
					v.major = smaller(num)
				} else {
					v.major = sameSmaller(num)
				}
			case sameBiggerMark:
				v.major = sameBigger(num)
			case sameSmallerMark:
				v.major = sameSmaller(num)
			default:
				v.major = same(num)
			}
		case 1:
			switch typ {
			case biggerMark:
				if len(splits) == 2 {
					v.minor = bigger(num)
				} else {
					v.minor = sameBigger(num)
				}
			case smallerMark:
				if len(splits) == 2 {
					v.minor = smaller(num)
				} else {
					v.minor = sameSmaller(num)
				}
			case sameBiggerMark:
				v.minor = sameBigger(num)
			case sameSmallerMark:
				v.minor = sameSmaller(num)
			default:
				v.minor = same(num)
			}
		case 2:
			switch typ {
			case emptyMark:
				v.patch = same(num)
			case tiledeMark:
				v.patch = sameBigger(num)
			case hatMark:
				if v.major == same(0) && v.minor == same(0) {
					v.patch = same(num)
				} else {
					v.patch = sameBigger(num)
				}
			case biggerMark:
				v.patch = bigger(num)
			case smallerMark:
				v.patch = smaller(num)
			case sameBiggerMark:
				v.patch = sameBigger(num)
			case sameSmallerMark:
				v.patch = sameSmaller(num)
			}
		}
	}

	return v, nil
}

func compareVersionString(numberStrs []string, v *version) (bool, error) {
	if len(numberStrs) != 3 {
		return false, fmt.Errorf("%v is invalid format", numberStrs)
	}
	major, err := strconv.ParseInt(numberStrs[0], 10, 64)
	if err != nil {
		return false, err
	}
	minor, err := strconv.ParseInt(numberStrs[1], 10, 64)
	if err != nil {
		return false, err
	}
	patch, err := strconv.ParseInt(numberStrs[2], 10, 64)
	if err != nil {
		return false, err
	}

	match, confirm := compareVersion(int(major), v.major)
	if confirm {
		return true, nil
	}
	if !match {
		return false, nil
	}
	match, confirm = compareVersion(int(minor), v.minor)
	if confirm {
		return true, nil
	}
	if !match {
		return false, nil
	}
	match, _ = compareVersion(int(patch), v.patch)

	return match, nil
}

func mustParse(numberStr string) int {
	num, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		panic(err)
	}
	return int(num)
}
