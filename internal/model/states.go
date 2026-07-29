package model

// State/province data for the two countries whose job locations routinely name
// one: the United States and Canada. It powers the third level of the board's
// location drill-down (region → country → state), which is what makes North
// America browsable — the region has only two countries but fifty-odd states.
//
// These tables are only consulted once a location has already resolved to the
// US or Canada, so codes that are ambiguous at the country level ("CA", "IN",
// "DE", "ON") are unambiguous here.

// usStateCodes maps every US state/territory code to its full name.
var usStateCodes = map[string]string{
	"al": "Alabama", "ak": "Alaska", "az": "Arizona", "ar": "Arkansas",
	"ca": "California", "co": "Colorado", "ct": "Connecticut", "de": "Delaware",
	"dc": "District of Columbia", "fl": "Florida", "ga": "Georgia", "hi": "Hawaii",
	"id": "Idaho", "il": "Illinois", "in": "Indiana", "ia": "Iowa", "ks": "Kansas",
	"ky": "Kentucky", "la": "Louisiana", "me": "Maine", "md": "Maryland",
	"ma": "Massachusetts", "mi": "Michigan", "mn": "Minnesota", "ms": "Mississippi",
	"mo": "Missouri", "mt": "Montana", "ne": "Nebraska", "nv": "Nevada",
	"nh": "New Hampshire", "nj": "New Jersey", "nm": "New Mexico", "ny": "New York",
	"nc": "North Carolina", "nd": "North Dakota", "oh": "Ohio", "ok": "Oklahoma",
	"or": "Oregon", "pa": "Pennsylvania", "pr": "Puerto Rico", "ri": "Rhode Island",
	"sc": "South Carolina", "sd": "South Dakota", "tn": "Tennessee", "tx": "Texas",
	"ut": "Utah", "vt": "Vermont", "va": "Virginia", "wa": "Washington",
	"wv": "West Virginia", "wi": "Wisconsin", "wy": "Wyoming",
}

// caProvinceCodes maps Canadian province/territory codes to their full name.
var caProvinceCodes = map[string]string{
	"ab": "Alberta", "bc": "British Columbia", "mb": "Manitoba", "nb": "New Brunswick",
	"nl": "Newfoundland and Labrador", "ns": "Nova Scotia", "nt": "Northwest Territories",
	"nu": "Nunavut", "on": "Ontario", "pe": "Prince Edward Island", "qc": "Quebec",
	"sk": "Saskatchewan", "yt": "Yukon",
}

// usCityStates maps the US cities in countryTable to their state, so a bare
// "Seattle" or "Palo Alto" still lands in a state.
var usCityStates = map[string]string{
	"new york": "New York", "new york city": "New York", "nyc": "New York",
	"brooklyn": "New York", "manhattan": "New York", "queens": "New York",
	"buffalo": "New York", "rochester": "New York", "syracuse": "New York",
	"albany":        "New York",
	"san francisco": "California", "bay area": "California", "palo alto": "California",
	"mountain view": "California", "sunnyvale": "California", "santa clara": "California",
	"san jose": "California", "cupertino": "California", "menlo park": "California",
	"redwood city": "California", "oakland": "California", "berkeley": "California",
	"san mateo": "California", "fremont": "California", "foster city": "California",
	"san bruno": "California", "south san francisco": "California", "pleasanton": "California",
	"san ramon": "California", "santa cruz": "California", "los angeles": "California",
	"santa monica": "California", "culver city": "California", "el segundo": "California",
	"playa vista": "California", "burbank": "California", "pasadena": "California",
	"long beach": "California", "irvine": "California", "san diego": "California",
	"sacramento": "California", "folsom": "California", "santa barbara": "California",
	"seattle": "Washington", "bellevue": "Washington", "redmond": "Washington",
	"kirkland": "Washington",
	"portland": "Oregon", "beaverton": "Oregon", "hillsboro": "Oregon",
	"denver": "Colorado", "boulder": "Colorado",
	"austin": "Texas", "dallas": "Texas", "houston": "Texas", "san antonio": "Texas",
	"fort worth": "Texas", "plano": "Texas",
	"chicago": "Illinois", "evanston": "Illinois",
	"boston": "Massachusetts", "somerville": "Massachusetts", "waltham": "Massachusetts",
	"atlanta": "Georgia",
	"miami":   "Florida", "orlando": "Florida", "tampa": "Florida",
	"charlotte": "North Carolina", "raleigh": "North Carolina", "durham": "North Carolina",
	"nashville": "Tennessee", "memphis": "Tennessee",
	"pittsburgh": "Pennsylvania", "philadelphia": "Pennsylvania",
	"arlington": "Virginia", "reston": "Virginia", "mclean": "Virginia",
	"herndon": "Virginia", "richmond": "Virginia", "virginia beach": "Virginia",
	"bethesda": "Maryland", "baltimore": "Maryland",
	"detroit": "Michigan", "ann arbor": "Michigan",
	"minneapolis": "Minnesota", "st. paul": "Minnesota",
	"st. louis": "Missouri", "st louis": "Missouri", "kansas city": "Missouri",
	"columbus": "Ohio", "cleveland": "Ohio", "cincinnati": "Ohio",
	"indianapolis": "Indiana",
	"madison":      "Wisconsin", "milwaukee": "Wisconsin",
	"phoenix": "Arizona", "tempe": "Arizona", "scottsdale": "Arizona",
	"chandler": "Arizona", "tucson": "Arizona",
	"salt lake city": "Utah", "lehi": "Utah", "provo": "Utah",
	"las vegas": "Nevada", "reno": "Nevada",
	"boise": "Idaho", "albuquerque": "New Mexico",
	"oklahoma city": "Oklahoma", "tulsa": "Oklahoma",
	"new orleans": "Louisiana", "louisville": "Kentucky",
	"hartford": "Connecticut", "stamford": "Connecticut",
	"princeton": "New Jersey", "newark": "New Jersey", "jersey city": "New Jersey",
	"hoboken":    "New Jersey",
	"providence": "Rhode Island", "des moines": "Iowa", "omaha": "Nebraska",
	"honolulu": "Hawaii", "anchorage": "Alaska",
}

// caCityProvinces maps the Canadian cities in countryTable to their province.
var caCityProvinces = map[string]string{
	"toronto": "Ontario", "mississauga": "Ontario", "markham": "Ontario",
	"ottawa": "Ontario", "waterloo": "Ontario", "kitchener": "Ontario",
	"vancouver": "British Columbia", "burnaby": "British Columbia",
	"montreal": "Quebec", "montréal": "Quebec", "quebec city": "Quebec",
	"calgary": "Alberta", "edmonton": "Alberta",
	"winnipeg": "Manitoba", "halifax": "Nova Scotia",
}

// Derived name→name lookups ("texas" → "Texas"), built in init.
var (
	usStateNames    map[string]string
	caProvinceNames map[string]string
)

func init() {
	usStateNames = make(map[string]string, len(usStateCodes))
	for _, name := range usStateCodes {
		usStateNames[normPhrase(name)] = name
	}
	caProvinceNames = make(map[string]string, len(caProvinceCodes))
	for _, name := range caProvinceCodes {
		caProvinceNames[normPhrase(name)] = name
	}
	// Spellings that differ from the canonical name.
	usStateNames["washington dc"] = "District of Columbia"
	usStateNames["washington d.c."] = "District of Columbia"
	caProvinceNames["québec"] = "Quebec"
	caProvinceNames["newfoundland"] = "Newfoundland and Labrador"
}

// stateSpec resolves a spelled-out US state or Canadian province name (as a
// crawler -region value, say). Codes are deliberately not accepted: "CA" and
// "IN" would be a coin flip between California/Canada and Indiana/India.
func stateSpec(s string) (string, bool) {
	if v, ok := usStateNames[s]; ok {
		return v, true
	}
	if v, ok := caProvinceNames[s]; ok {
		return v, true
	}
	return "", false
}

// statesOf returns the code/name/city lookups for a country's subdivisions, or
// nils for countries the board does not track at state level.
func statesOf(country string) (codes, names, cities map[string]string) {
	switch country {
	case "United States":
		return usStateCodes, usStateNames, usCityStates
	case "Canada":
		return caProvinceCodes, caProvinceNames, caCityProvinces
	}
	return nil, nil, nil
}
