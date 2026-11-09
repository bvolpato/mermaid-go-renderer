//go:build domcheck

package dom

// checkSimpleParser makes ParseHTMLDocument compare every document of the
// simple parser with the document of the full parser. Run the tests with
// -tags domcheck to prove the simple parser on everything that they parse.
const checkSimpleParser = true
