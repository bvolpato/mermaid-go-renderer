// A port of the part of the WHATWG URL Standard (https://url.spec.whatwg.org)
// that sanitizeUrl of @braintree/sanitize-url reaches through
// `new URL(value)` and `url.toString()`: the basic URL parser with no base
// URL for the schemes http and https, the host parser, and the URL
// serializer. The names follow the standard.
//
// The reference is Chromium, which differs from the standard in four places
// (measured in Chromium 156, see testdata/urls.json.gz):
//
//   - a "|" in a path is percent-encoded
//   - a "'" in the user name or the password is percent-encoded
//   - a host of ASCII characters is not checked as an internationalized
//     domain name, so an invalid "xn--" label is kept
//   - a space and a "*" in a host are percent-encoded, where the standard
//     rejects the space and keeps the "*"

package sanitize

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// eof is the EOF code point of the URL parser.
const eof rune = -1

// Percent-encode sets. Every set also has the C0 controls and all code points
// above U+007E (the C0 control percent-encode set).
const (
	fragmentPercentEncodeSet     = " \"<>`"
	queryPercentEncodeSet        = " \"#<>"
	specialQueryPercentEncodeSet = queryPercentEncodeSet + "'"
	pathPercentEncodeSet         = queryPercentEncodeSet + "?^`{}" + "|"
	userinfoPercentEncodeSet     = pathPercentEncodeSet + "/:;=@[\\]|" + "'"
)

// utf8PercentEncode is "UTF-8 percent-encode a code point using a
// percent-encode set", appended to sb.
func utf8PercentEncode(sb *strings.Builder, c rune, set string) {
	const hex = "0123456789ABCDEF"
	if c >= 0x20 && c <= 0x7E && strings.IndexByte(set, byte(c)) < 0 {
		sb.WriteByte(byte(c))
		return
	}
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], c)
	for _, b := range buf[:n] {
		sb.WriteByte('%')
		sb.WriteByte(hex[b>>4])
		sb.WriteByte(hex[b&0xF])
	}
}

// codePoints returns the code points of s. A lone surrogate, which this
// module keeps in three bytes, and every other invalid byte sequence become
// U+FFFD, as the UTF-8 encoder of a browser does.
func codePoints(s string) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			out = append(out, rune(c))
			i++
			continue
		}
		if c == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2]&0xC0 == 0x80 {
			out = append(out, utf8.RuneError)
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		out = append(out, r)
		i += size
	}
	return out
}

// urlRecord is a URL record for a special scheme.
type urlRecord struct {
	scheme      string
	username    string
	password    string
	host        string
	port        int // -1 is null
	path        []string
	query       string
	hasQuery    bool
	fragment    string
	hasFragment bool
}

func defaultPort(scheme string) int {
	switch scheme {
	case "http":
		return 80
	case "https":
		return 443
	}
	return -1
}

func isASCIIAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c rune) bool { return c >= '0' && c <= '9' }

// isSingleDotSegment is "single-dot URL path segment".
func isSingleDotSegment(s string) bool {
	return s == "." || strings.EqualFold(s, "%2e")
}

// isDoubleDotSegment is "double-dot URL path segment".
func isDoubleDotSegment(s string) bool {
	switch strings.ToLower(s) {
	case "..", ".%2e", "%2e.", "%2e%2e":
		return true
	}
	return false
}

// parseHTTPURL is new URL(input).href for an input whose scheme is http or
// https. The second result is false when the URL parser fails, and for every
// other scheme. sanitizeUrl calls it only for a value that starts with
// "http:" or "https:", or that cannot parse at all.
func parseHTTPURL(input string) (string, bool) {
	// Remove any leading and trailing C0 control or space from input.
	input = strings.TrimFunc(input, func(r rune) bool { return r <= 0x20 })
	// Remove all ASCII tab or newline from input.
	if strings.ContainsAny(input, "\t\n\r") {
		input = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(input)
	}
	in := codePoints(input)
	at := func(p int) rune {
		if p >= len(in) {
			return eof
		}
		return in[p]
	}

	// Scheme start state and scheme state.
	p := 0
	if !isASCIIAlpha(at(p)) {
		return "", false
	}
	for ; ; p++ {
		c := at(p)
		if isASCIIAlpha(c) || isASCIIDigit(c) || c == '+' || c == '-' || c == '.' {
			continue
		}
		if c != ':' {
			// No scheme state with no base URL: failure.
			return "", false
		}
		break
	}
	url := &urlRecord{scheme: strings.ToLower(string(in[:p])), port: -1}
	if url.scheme != "http" && url.scheme != "https" {
		return "", false
	}
	p++

	// Special authority slashes state.
	if at(p) == '/' && at(p+1) == '/' {
		p += 2
	}
	// Special authority ignore slashes state.
	for at(p) == '/' || at(p) == '\\' {
		p++
	}

	// Authority state.
	{
		var buffer []rune
		var username, password strings.Builder
		atSignSeen, passwordTokenSeen := false, false
		for ; ; p++ {
			c := at(p)
			if c == '@' {
				if atSignSeen {
					buffer = append([]rune("%40"), buffer...)
				}
				atSignSeen = true
				for _, codePoint := range buffer {
					if codePoint == ':' && !passwordTokenSeen {
						passwordTokenSeen = true
						continue
					}
					if passwordTokenSeen {
						utf8PercentEncode(&password, codePoint, userinfoPercentEncodeSet)
					} else {
						utf8PercentEncode(&username, codePoint, userinfoPercentEncodeSet)
					}
				}
				buffer = buffer[:0]
				continue
			}
			if c == eof || c == '/' || c == '?' || c == '#' || c == '\\' {
				if atSignSeen && len(buffer) == 0 {
					// host-missing validation error.
					return "", false
				}
				p -= len(buffer)
				break
			}
			buffer = append(buffer, c)
		}
		url.username, url.password = username.String(), password.String()
	}

	// Host state.
	{
		var buffer []rune
		insideBrackets := false
		portState := false
		for ; ; p++ {
			c := at(p)
			if c == ':' && !insideBrackets {
				if len(buffer) == 0 {
					return "", false
				}
				portState = true
				p++
				break
			}
			if c == eof || c == '/' || c == '?' || c == '#' || c == '\\' {
				if len(buffer) == 0 {
					return "", false
				}
				break
			}
			switch c {
			case '[':
				insideBrackets = true
			case ']':
				insideBrackets = false
			}
			buffer = append(buffer, c)
		}
		host, ok := hostParser(string(buffer))
		if !ok {
			return "", false
		}
		url.host = host

		// Port state.
		if portState {
			port, digits := 0, 0
			for ; ; p++ {
				c := at(p)
				if isASCIIDigit(c) {
					digits++
					if port <= 65535 {
						port = port*10 + int(c-'0')
					}
					continue
				}
				if c == eof || c == '/' || c == '?' || c == '#' || c == '\\' {
					if digits > 0 {
						if port > 65535 {
							return "", false
						}
						if port != defaultPort(url.scheme) {
							url.port = port
						}
					}
					break
				}
				return "", false
			}
		}
	}

	// Path start state.
	if c := at(p); c == '/' || c == '\\' {
		p++
	}
	// Path state.
	{
		var buffer strings.Builder
		for ; ; p++ {
			c := at(p)
			if c == eof || c == '/' || c == '\\' || c == '?' || c == '#' {
				segment := buffer.String()
				slash := c == '/' || c == '\\'
				switch {
				case isDoubleDotSegment(segment):
					// Shorten the path.
					if len(url.path) > 0 {
						url.path = url.path[:len(url.path)-1]
					}
					if !slash {
						url.path = append(url.path, "")
					}
				case isSingleDotSegment(segment):
					if !slash {
						url.path = append(url.path, "")
					}
				default:
					url.path = append(url.path, segment)
				}
				buffer.Reset()
				if !slash {
					break
				}
				continue
			}
			utf8PercentEncode(&buffer, c, pathPercentEncodeSet)
		}
	}

	// Query state.
	if at(p) == '?' {
		p++
		url.hasQuery = true
		var query strings.Builder
		for ; at(p) != eof && at(p) != '#'; p++ {
			utf8PercentEncode(&query, at(p), specialQueryPercentEncodeSet)
		}
		url.query = query.String()
	}
	// Fragment state.
	if at(p) == '#' {
		p++
		url.hasFragment = true
		var fragment strings.Builder
		for ; at(p) != eof; p++ {
			utf8PercentEncode(&fragment, at(p), fragmentPercentEncodeSet)
		}
		url.fragment = fragment.String()
	}
	return url.serialize(), true
}

// serialize is the URL serializer.
func (url *urlRecord) serialize() string {
	var sb strings.Builder
	sb.WriteString(url.scheme)
	sb.WriteString("://")
	if url.username != "" || url.password != "" {
		sb.WriteString(url.username)
		if url.password != "" {
			sb.WriteByte(':')
			sb.WriteString(url.password)
		}
		sb.WriteByte('@')
	}
	sb.WriteString(url.host)
	if url.port >= 0 {
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(url.port))
	}
	for _, segment := range url.path {
		sb.WriteByte('/')
		sb.WriteString(segment)
	}
	if url.hasQuery {
		sb.WriteByte('?')
		sb.WriteString(url.query)
	}
	if url.hasFragment {
		sb.WriteByte('#')
		sb.WriteString(url.fragment)
	}
	return sb.String()
}

// percentDecode is "percent-decode a byte sequence".
func percentDecode(s string) string {
	if strings.IndexByte(s, '%') < 0 {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if b, ok := percentByte(s, i); ok {
			sb.WriteByte(b)
			i += 2
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// forbiddenDomainCodePoint is "forbidden domain code point", without the
// space: Chromium escapes a space in a host.
func forbiddenDomainCodePoint(c byte) bool {
	if c < 0x20 || c == 0x7F {
		return true
	}
	return strings.IndexByte("#/:<>?@[\\]^|%", c) >= 0
}

// idnaProfile is the Unicode ToASCII operation with the flags of "domain to
// ASCII" for beStrict false: CheckHyphens false, CheckBidi true,
// CheckJoiners true, UseSTD3ASCIIRules false, Transitional_Processing false,
// VerifyDnsLength false.
var idnaProfile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	idna.CheckJoiners(true),
	idna.CheckHyphens(false),
	idna.StrictDomainName(false),
	idna.Transitional(false),
	idna.VerifyDNSLength(false),
)

// domainToASCII is "domain to ASCII" with beStrict false.
func domainToASCII(domain string) (string, bool) {
	// For an ASCII domain with no label that starts with "xn--", the
	// operation is ASCII lowercasing. Chromium also only lowercases an ASCII
	// domain that has such a label: it does not check the Punycode.
	var result string
	if isASCII(domain) {
		result = strings.ToLower(domain)
	} else {
		var err error
		result, err = idnaProfile.ToASCII(domain)
		if err != nil {
			return "", false
		}
	}
	if result == "" {
		return "", false
	}
	return result, true
}

// hostParser is the host parser for a special scheme (isOpaque false). It
// returns the serialized host.
func hostParser(input string) (string, bool) {
	if strings.HasPrefix(input, "[") {
		if !strings.HasSuffix(input, "]") {
			return "", false
		}
		address, ok := ipv6Parser(input[1 : len(input)-1])
		if !ok {
			return "", false
		}
		return "[" + ipv6Serializer(address) + "]", true
	}
	// UTF-8 decode without BOM of the percent-decoded input.
	domain := strings.ToValidUTF8(percentDecode(input), string(utf8.RuneError))
	domain = strings.TrimPrefix(domain, string(rune(0xFEFF)))
	asciiDomain, ok := domainToASCII(domain)
	if !ok {
		return "", false
	}
	escape := false
	for i := 0; i < len(asciiDomain); i++ {
		c := asciiDomain[i]
		if forbiddenDomainCodePoint(c) {
			return "", false
		}
		escape = escape || c == ' ' || c == '*'
	}
	if endsInANumber(asciiDomain) {
		address, ok := ipv4Parser(asciiDomain)
		if !ok {
			return "", false
		}
		return ipv4Serializer(address), true
	}
	if escape {
		asciiDomain = strings.NewReplacer(" ", "%20", "*", "%2A").Replace(asciiDomain)
	}
	return asciiDomain, true
}

// endsInANumber is the "ends in a number checker".
func endsInANumber(input string) bool {
	parts := strings.Split(input, ".")
	if parts[len(parts)-1] == "" {
		if len(parts) == 1 {
			return false
		}
		parts = parts[:len(parts)-1]
	}
	last := parts[len(parts)-1]
	if last != "" {
		digits := true
		for i := 0; i < len(last); i++ {
			if last[i] < '0' || last[i] > '9' {
				digits = false
				break
			}
		}
		if digits {
			return true
		}
	}
	_, ok := ipv4NumberParser(last)
	return ok
}

// ipv4NumberParser is the IPv4 number parser. The value is capped: every
// caller only compares it with limits below 2^32.
func ipv4NumberParser(input string) (uint64, bool) {
	if input == "" {
		return 0, false
	}
	radix := uint64(10)
	if len(input) >= 2 && (input[:2] == "0x" || input[:2] == "0X") {
		input = input[2:]
		radix = 16
	} else if len(input) >= 2 && input[0] == '0' {
		input = input[1:]
		radix = 8
	}
	if input == "" {
		return 0, true
	}
	const limit = uint64(1) << 40
	var value uint64
	for i := 0; i < len(input); i++ {
		d, ok := hexValue(input[i])
		if !ok || uint64(d) >= radix {
			return 0, false
		}
		if value < limit {
			value = value*radix + uint64(d)
		}
	}
	return value, true
}

// ipv4Parser is the IPv4 parser.
func ipv4Parser(input string) (uint32, bool) {
	parts := strings.Split(input, ".")
	if parts[len(parts)-1] == "" {
		if len(parts) > 1 {
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) > 4 {
		return 0, false
	}
	numbers := make([]uint64, 0, 4)
	for _, part := range parts {
		n, ok := ipv4NumberParser(part)
		if !ok {
			return 0, false
		}
		numbers = append(numbers, n)
	}
	for _, n := range numbers[:len(numbers)-1] {
		if n > 255 {
			return 0, false
		}
	}
	last := numbers[len(numbers)-1]
	if last >= uint64(1)<<(8*uint(5-len(numbers))) {
		return 0, false
	}
	ipv4 := last
	for counter, n := range numbers[:len(numbers)-1] {
		ipv4 += n << (8 * uint(3-counter))
	}
	return uint32(ipv4), true
}

// ipv4Serializer is the IPv4 serializer.
func ipv4Serializer(address uint32) string {
	return strconv.Itoa(int(address>>24)) + "." + strconv.Itoa(int(address>>16&0xFF)) + "." +
		strconv.Itoa(int(address>>8&0xFF)) + "." + strconv.Itoa(int(address&0xFF))
}

// ipv6Parser is the IPv6 parser.
func ipv6Parser(input string) ([8]uint16, bool) {
	var address [8]uint16
	pieceIndex := 0
	compress := -1
	in := []byte(input)
	at := func(p int) int {
		if p >= len(in) {
			return -1
		}
		return int(in[p])
	}
	p := 0
	if at(p) == ':' {
		if at(p+1) != ':' {
			return address, false
		}
		p += 2
		pieceIndex++
		compress = pieceIndex
	}
	for at(p) != -1 {
		if pieceIndex == 8 {
			return address, false
		}
		if at(p) == ':' {
			if compress != -1 {
				return address, false
			}
			p++
			pieceIndex++
			compress = pieceIndex
			continue
		}
		value, length := 0, 0
		for length < 4 && at(p) != -1 {
			d, ok := hexValue(byte(at(p)))
			if !ok {
				break
			}
			value = value*0x10 + int(d)
			p++
			length++
		}
		if at(p) == '.' {
			if length == 0 {
				return address, false
			}
			p -= length
			if pieceIndex > 6 {
				return address, false
			}
			numbersSeen := 0
			for at(p) != -1 {
				ipv4Piece := -1
				if numbersSeen > 0 {
					if at(p) == '.' && numbersSeen < 4 {
						p++
					} else {
						return address, false
					}
				}
				if c := at(p); c < '0' || c > '9' {
					return address, false
				}
				for c := at(p); c >= '0' && c <= '9'; c = at(p) {
					number := c - '0'
					switch {
					case ipv4Piece == -1:
						ipv4Piece = number
					case ipv4Piece == 0:
						return address, false
					default:
						ipv4Piece = ipv4Piece*10 + number
					}
					if ipv4Piece > 255 {
						return address, false
					}
					p++
				}
				address[pieceIndex] = address[pieceIndex]*0x100 + uint16(ipv4Piece)
				numbersSeen++
				if numbersSeen == 2 || numbersSeen == 4 {
					pieceIndex++
				}
			}
			if numbersSeen != 4 {
				return address, false
			}
			break
		} else if at(p) == ':' {
			p++
			if at(p) == -1 {
				return address, false
			}
		} else if at(p) != -1 {
			return address, false
		}
		address[pieceIndex] = uint16(value)
		pieceIndex++
	}
	if compress != -1 {
		swaps := pieceIndex - compress
		pieceIndex = 7
		for pieceIndex != 0 && swaps > 0 {
			address[pieceIndex], address[compress+swaps-1] = address[compress+swaps-1], address[pieceIndex]
			pieceIndex--
			swaps--
		}
	} else if pieceIndex != 8 {
		return address, false
	}
	return address, true
}

// ipv6Serializer is the IPv6 serializer.
func ipv6Serializer(address [8]uint16) string {
	// Find the first longest sequence of two or more pieces that are 0.
	compress, longest := -1, 1
	for i := 0; i < 8; {
		if address[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && address[j] == 0 {
			j++
		}
		if j-i > longest {
			compress, longest = i, j-i
		}
		i = j
	}
	var sb strings.Builder
	ignore0 := false
	for pieceIndex := 0; pieceIndex < 8; pieceIndex++ {
		if ignore0 && address[pieceIndex] == 0 {
			continue
		} else if ignore0 {
			ignore0 = false
		}
		if compress == pieceIndex {
			if pieceIndex == 0 {
				sb.WriteString("::")
			} else {
				sb.WriteByte(':')
			}
			ignore0 = true
			continue
		}
		sb.WriteString(strconv.FormatUint(uint64(address[pieceIndex]), 16))
		if pieceIndex != 7 {
			sb.WriteByte(':')
		}
	}
	return sb.String()
}
