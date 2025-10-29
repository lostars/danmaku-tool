package iqiyi

import "strconv"

const xorKey = 0x75706971676c
const segmentInterval = 60
const segmentSalt = "cbzuw1259a"

func parseToNumberId(id string) int64 {
	num, err := strconv.ParseInt(id, 36, 64)
	if err != nil {
		return 0
	}
	numBinary := strconv.FormatInt(num, 2)
	keyBinary := strconv.FormatInt(int64(xorKey), 2)

	numBits := reverseString(numBinary)
	keyBits := reverseString(keyBinary)

	maxLen := len(numBits)
	if len(keyBits) > maxLen {
		maxLen = len(keyBits)
	}

	resultBits := make([]byte, 0, maxLen)

	for i := 0; i < maxLen; i++ {
		var numBit, keyBit byte = '0', '0'
		if i < len(numBits) {
			numBit = numBits[i]
		}
		if i < len(keyBits) {
			keyBit = keyBits[i]
		}
		if numBit == keyBit {
			resultBits = append(resultBits, '0')
		} else {
			resultBits = append(resultBits, '1')
		}
	}

	finalBinary := reverseBytes(resultBits)

	if len(finalBinary) == 0 {
		return 0
	}
	val, _ := strconv.ParseInt(string(finalBinary), 2, 64)
	if val < 900000 {
		val = 100 * (val + 900000)
	}
	return val
}

func reverseString(s string) string {
	r := []byte(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func reverseBytes(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}
