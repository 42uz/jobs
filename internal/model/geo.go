package model

import "sort"

// This file is the single source of truth for geography: every country the
// board recognizes, the region it belongs to, its ISO codes, its name variants
// and its major cities. location.go derives all of its matching from the tables
// built here, and the server uses CountryOf/RegionOf to build its facets.

// Canonical region names. These are the values exposed by RegionOf and by the
// /api/jobs region facet.
const (
	RegionEurope       = "Europe"
	RegionNorthAmerica = "North America"
	RegionLatinAmerica = "Latin America"
	RegionAsia         = "Asia"
	RegionMiddleEast   = "Middle East"
	RegionAfrica       = "Africa"
	RegionOceania      = "Oceania"
)

// Regions lists the canonical regions in display order.
var Regions = []string{
	RegionEurope, RegionNorthAmerica, RegionLatinAmerica,
	RegionAsia, RegionMiddleEast, RegionAfrica, RegionOceania,
}

// country describes one recognized country. cities are lowercase and must be
// unique across the whole table: where a city name is shared by several
// countries (Cambridge, Birmingham, Vancouver, Santiago, …) it is listed once,
// for the dominant interpretation, and the other one is resolved by its
// state/province code instead ("Cambridge, MA" → United States).
type country struct {
	name     string
	region   string
	iso2     string
	iso3     string
	variants []string // extra lowercase name spellings (the canonical name is added automatically)
	cities   []string
}

var countryTable = []country{
	// ── Europe ────────────────────────────────────────────────────────────
	{name: "United Kingdom", region: RegionEurope, iso2: "gb", iso3: "gbr",
		variants: []string{"uk", "u.k.", "great britain", "england", "scotland", "wales", "northern ireland", "britain"},
		cities:   []string{"london", "greater london", "edinburgh", "glasgow", "manchester", "birmingham", "cambridge", "oxford", "bristol", "leeds", "reading", "belfast", "cardiff", "nottingham", "sheffield", "liverpool", "brighton", "dundee", "newcastle", "milton keynes", "swindon"}},
	{name: "Ireland", region: RegionEurope, iso2: "ie", iso3: "irl",
		cities: []string{"dublin", "cork", "galway", "limerick"}},
	{name: "Germany", region: RegionEurope, iso2: "de", iso3: "deu",
		variants: []string{"deutschland"},
		cities:   []string{"berlin", "munich", "münchen", "muenchen", "hamburg", "frankfurt", "cologne", "köln", "koeln", "stuttgart", "düsseldorf", "dusseldorf", "nuremberg", "nürnberg", "dresden", "leipzig", "hannover", "bonn", "mannheim", "karlsruhe", "freiburg", "eschborn", "walldorf", "ismaning", "aachen", "dortmund", "essen", "bremen", "duisburg", "bochum", "wuppertal", "bielefeld", "münster", "augsburg", "mainz", "wiesbaden", "braunschweig", "ulm", "regensburg", "erlangen", "heidelberg", "darmstadt", "kiel"}},
	{name: "France", region: RegionEurope, iso2: "fr", iso3: "fra",
		cities: []string{"paris", "lyon", "toulouse", "rennes", "nantes", "bordeaux", "lille", "nice", "marseille", "grenoble", "montpellier", "strasbourg", "sophia antipolis", "issy-les-moulineaux"}},
	{name: "Netherlands", region: RegionEurope, iso2: "nl", iso3: "nld",
		variants: []string{"nederland", "the netherlands", "holland"},
		cities:   []string{"amsterdam", "rotterdam", "utrecht", "eindhoven", "the hague", "den haag", "groningen", "tilburg", "haarlem", "veldhoven", "amersfoort", "drachten", "hilversum", "delft"}},
	{name: "Spain", region: RegionEurope, iso2: "es", iso3: "esp",
		variants: []string{"españa", "espana"},
		cities:   []string{"madrid", "barcelona", "valencia", "málaga", "malaga", "sevilla", "seville", "bilbao", "zaragoza", "palma", "alicante", "alcobendas", "san sebastián"}},
	{name: "Portugal", region: RegionEurope, iso2: "pt", iso3: "prt",
		cities: []string{"lisbon", "lisboa", "porto", "braga", "coimbra"}},
	{name: "Italy", region: RegionEurope, iso2: "it", iso3: "ita",
		variants: []string{"italia"},
		cities:   []string{"milan", "milano", "rome", "roma", "turin", "torino", "bologna", "florence", "firenze", "naples", "napoli", "venice", "venezia", "genoa", "genova", "padova"}},
	{name: "Switzerland", region: RegionEurope, iso2: "ch", iso3: "che",
		variants: []string{"schweiz", "suisse", "svizzera"},
		cities:   []string{"zurich", "zürich", "geneva", "genève", "basel", "lausanne", "bern", "zug", "lugano"}},
	{name: "Austria", region: RegionEurope, iso2: "at", iso3: "aut",
		variants: []string{"österreich", "oesterreich"},
		cities:   []string{"vienna", "wien", "graz", "linz", "salzburg", "innsbruck"}},
	{name: "Belgium", region: RegionEurope, iso2: "be", iso3: "bel",
		variants: []string{"belgië", "belgie", "belgique"},
		cities:   []string{"brussels", "bruxelles", "antwerp", "antwerpen", "ghent", "gent", "bruges", "brugge", "liège", "liege", "leuven", "mechelen"}},
	{name: "Sweden", region: RegionEurope, iso2: "se", iso3: "swe",
		variants: []string{"sverige"},
		cities:   []string{"stockholm", "gothenburg", "göteborg", "goteborg", "malmö", "malmo", "uppsala", "lund", "linköping"}},
	{name: "Denmark", region: RegionEurope, iso2: "dk", iso3: "dnk",
		variants: []string{"danmark"},
		cities:   []string{"copenhagen", "københavn", "kobenhavn", "aarhus", "roskilde", "odense", "billund"}},
	{name: "Norway", region: RegionEurope, iso2: "no", iso3: "nor",
		variants: []string{"norge"},
		cities:   []string{"oslo", "bergen", "trondheim", "stavanger"}},
	{name: "Finland", region: RegionEurope, iso2: "fi", iso3: "fin",
		variants: []string{"suomi"},
		cities:   []string{"helsinki", "espoo", "tampere", "oulu", "turku"}},
	{name: "Iceland", region: RegionEurope, iso2: "is", iso3: "isl",
		variants: []string{"ísland"},
		cities:   []string{"reykjavik", "reykjavík"}},
	{name: "Poland", region: RegionEurope, iso2: "pl", iso3: "pol",
		variants: []string{"polska"},
		cities:   []string{"warsaw", "warszawa", "kraków", "krakow", "wrocław", "wroclaw", "gdańsk", "gdansk", "poznań", "poznan", "katowice", "łódź", "lodz", "szczecin", "gdynia", "rzeszów"}},
	{name: "Czechia", region: RegionEurope, iso2: "cz", iso3: "cze",
		variants: []string{"czech republic", "czech"},
		cities:   []string{"prague", "praha", "brno", "ostrava"}},
	{name: "Slovakia", region: RegionEurope, iso2: "sk", iso3: "svk",
		cities: []string{"bratislava", "košice", "kosice"}},
	{name: "Hungary", region: RegionEurope, iso2: "hu", iso3: "hun",
		variants: []string{"magyarország"},
		cities:   []string{"budapest", "debrecen", "szeged"}},
	{name: "Romania", region: RegionEurope, iso2: "ro", iso3: "rou",
		cities: []string{"bucharest", "bucurești", "bucuresti", "cluj", "cluj-napoca", "timișoara", "timisoara", "iași", "iasi", "brasov", "brașov"}},
	{name: "Bulgaria", region: RegionEurope, iso2: "bg", iso3: "bgr",
		cities: []string{"sofia", "plovdiv", "varna", "burgas"}},
	{name: "Greece", region: RegionEurope, iso2: "gr", iso3: "grc",
		variants: []string{"hellas"},
		cities:   []string{"athens", "thessaloniki", "patras"}},
	{name: "Croatia", region: RegionEurope, iso2: "hr", iso3: "hrv",
		variants: []string{"hrvatska"},
		cities:   []string{"zagreb", "split", "rijeka"}},
	{name: "Slovenia", region: RegionEurope, iso2: "si", iso3: "svn",
		cities: []string{"ljubljana", "maribor"}},
	{name: "Serbia", region: RegionEurope, iso2: "rs", iso3: "srb",
		cities: []string{"belgrade", "beograd", "novi sad", "niš"}},
	{name: "Bosnia and Herzegovina", region: RegionEurope, iso2: "ba", iso3: "bih",
		variants: []string{"bosnia", "bosnia & herzegovina"},
		cities:   []string{"sarajevo", "banja luka"}},
	{name: "North Macedonia", region: RegionEurope, iso2: "mk", iso3: "mkd",
		variants: []string{"macedonia"},
		cities:   []string{"skopje"}},
	{name: "Montenegro", region: RegionEurope, iso2: "me", iso3: "mne",
		cities: []string{"podgorica"}},
	{name: "Albania", region: RegionEurope, iso2: "al", iso3: "alb",
		cities: []string{"tirana", "tiranë"}},
	{name: "Kosovo", region: RegionEurope, iso2: "xk",
		cities: []string{"pristina", "prishtina"}},
	{name: "Moldova", region: RegionEurope, iso2: "md", iso3: "mda",
		cities: []string{"chisinau", "chișinău"}},
	{name: "Ukraine", region: RegionEurope, iso2: "ua", iso3: "ukr",
		cities: []string{"kyiv", "kiev", "lviv", "kharkiv", "odesa", "odessa", "dnipro"}},
	{name: "Belarus", region: RegionEurope, iso2: "by", iso3: "blr",
		cities: []string{"minsk"}},
	{name: "Lithuania", region: RegionEurope, iso2: "lt", iso3: "ltu",
		cities: []string{"vilnius", "kaunas"}},
	{name: "Latvia", region: RegionEurope, iso2: "lv", iso3: "lva",
		cities: []string{"riga"}},
	{name: "Estonia", region: RegionEurope, iso2: "ee", iso3: "est",
		cities: []string{"tallinn", "tartu"}},
	{name: "Luxembourg", region: RegionEurope, iso2: "lu", iso3: "lux",
		cities: []string{"luxembourg"}},
	{name: "Malta", region: RegionEurope, iso2: "mt", iso3: "mlt",
		cities: []string{"valletta", "sliema"}},
	{name: "Cyprus", region: RegionEurope, iso2: "cy", iso3: "cyp",
		cities: []string{"nicosia", "limassol"}},
	{name: "Monaco", region: RegionEurope, iso2: "mc", iso3: "mco"},
	{name: "Liechtenstein", region: RegionEurope, iso2: "li", iso3: "lie",
		cities: []string{"vaduz"}},
	{name: "Russia", region: RegionEurope, iso2: "ru", iso3: "rus",
		variants: []string{"russian federation"},
		cities:   []string{"moscow", "saint petersburg", "st petersburg", "novosibirsk"}},
	// Transcontinental countries whose tech hiring is normally grouped with
	// Europe/EMEA.
	{name: "Turkey", region: RegionEurope, iso2: "tr", iso3: "tur",
		variants: []string{"türkiye", "turkiye"},
		cities:   []string{"istanbul", "ankara", "izmir"}},
	{name: "Georgia", region: RegionEurope, iso2: "ge", iso3: "geo",
		cities: []string{"tbilisi"}}, // the country; the US state is handled in location.go
	{name: "Armenia", region: RegionEurope, iso2: "am", iso3: "arm",
		cities: []string{"yerevan"}},

	// ── North America ─────────────────────────────────────────────────────
	{name: "United States", region: RegionNorthAmerica, iso2: "us", iso3: "usa",
		variants: []string{"united states of america", "u.s.a", "u.s.", "usa", "us"},
		cities: []string{
			"new york", "new york city", "nyc", "brooklyn", "manhattan", "queens",
			"san francisco", "bay area", "palo alto", "mountain view", "sunnyvale", "santa clara",
			"san jose", "cupertino", "menlo park", "redwood city", "oakland", "berkeley", "san mateo",
			"fremont", "foster city", "san bruno", "south san francisco", "pleasanton", "san ramon",
			"santa cruz", "los angeles", "santa monica", "culver city", "el segundo", "playa vista",
			"burbank", "pasadena", "long beach", "irvine", "san diego", "sacramento", "folsom",
			"santa barbara", "seattle", "bellevue", "redmond", "kirkland", "portland", "beaverton",
			"hillsboro", "denver", "boulder", "austin", "dallas", "houston", "san antonio",
			"fort worth", "plano", "chicago", "evanston", "boston", "somerville", "waltham",
			"atlanta", "miami", "orlando", "tampa", "charlotte", "raleigh", "durham", "nashville",
			"pittsburgh", "philadelphia", "arlington", "reston", "mclean", "herndon", "bethesda",
			"baltimore", "detroit", "ann arbor", "minneapolis", "st. paul", "st. louis", "st louis",
			"kansas city", "columbus", "cleveland", "cincinnati", "indianapolis", "madison",
			"milwaukee", "phoenix", "tempe", "scottsdale", "chandler", "tucson", "salt lake city",
			"lehi", "provo", "las vegas", "reno", "boise", "albuquerque", "oklahoma city", "tulsa",
			"new orleans", "memphis", "louisville", "richmond", "virginia beach", "hartford",
			"stamford", "princeton", "newark", "jersey city", "hoboken", "buffalo", "rochester",
			"syracuse", "albany", "providence", "des moines", "omaha", "honolulu", "anchorage",
		}},
	{name: "Canada", region: RegionNorthAmerica, iso2: "ca", iso3: "can",
		cities: []string{"toronto", "vancouver", "montreal", "montréal", "ottawa", "calgary", "edmonton", "waterloo", "kitchener", "quebec city", "winnipeg", "halifax", "mississauga", "burnaby", "markham"}},

	// ── Latin America ─────────────────────────────────────────────────────
	{name: "Mexico", region: RegionLatinAmerica, iso2: "mx", iso3: "mex",
		variants: []string{"méxico"},
		cities:   []string{"mexico city", "ciudad de méxico", "guadalajara", "monterrey", "querétaro", "queretaro", "tijuana", "puebla"}},
	{name: "Brazil", region: RegionLatinAmerica, iso2: "br", iso3: "bra",
		variants: []string{"brasil"},
		cities:   []string{"são paulo", "sao paulo", "rio de janeiro", "belo horizonte", "curitiba", "porto alegre", "brasília", "brasilia", "recife", "campinas", "florianópolis", "florianopolis"}},
	{name: "Argentina", region: RegionLatinAmerica, iso2: "ar", iso3: "arg",
		cities: []string{"buenos aires", "córdoba", "cordoba", "rosario", "mendoza"}},
	{name: "Chile", region: RegionLatinAmerica, iso2: "cl", iso3: "chl",
		cities: []string{"santiago", "valparaíso", "valparaiso"}},
	{name: "Colombia", region: RegionLatinAmerica, iso2: "co", iso3: "col",
		cities: []string{"bogotá", "bogota", "medellín", "medellin", "cali", "barranquilla"}},
	{name: "Peru", region: RegionLatinAmerica, iso2: "pe", iso3: "per",
		cities: []string{"lima"}},
	{name: "Uruguay", region: RegionLatinAmerica, iso2: "uy", iso3: "ury",
		cities: []string{"montevideo"}},
	{name: "Costa Rica", region: RegionLatinAmerica, iso2: "cr", iso3: "cri",
		cities: []string{"heredia", "escazú", "escazu"}},
	{name: "Panama", region: RegionLatinAmerica, iso2: "pa", iso3: "pan",
		cities: []string{"panama city"}},
	{name: "Ecuador", region: RegionLatinAmerica, iso2: "ec", iso3: "ecu",
		cities: []string{"quito", "guayaquil"}},
	{name: "Bolivia", region: RegionLatinAmerica, iso2: "bo", iso3: "bol",
		cities: []string{"la paz", "santa cruz de la sierra"}},
	{name: "Paraguay", region: RegionLatinAmerica, iso2: "py", iso3: "pry",
		cities: []string{"asunción", "asuncion"}},
	{name: "Venezuela", region: RegionLatinAmerica, iso2: "ve", iso3: "ven",
		cities: []string{"caracas"}},
	{name: "Guatemala", region: RegionLatinAmerica, iso2: "gt", iso3: "gtm",
		cities: []string{"guatemala city"}},
	{name: "Dominican Republic", region: RegionLatinAmerica, iso2: "do", iso3: "dom",
		cities: []string{"santo domingo"}},
	{name: "Jamaica", region: RegionLatinAmerica, iso2: "jm", iso3: "jam",
		cities: []string{"kingston"}},
	{name: "Trinidad and Tobago", region: RegionLatinAmerica, iso2: "tt", iso3: "tto",
		variants: []string{"trinidad & tobago", "trinidad"},
		cities:   []string{"port of spain"}},

	// ── Asia ──────────────────────────────────────────────────────────────
	{name: "India", region: RegionAsia, iso2: "in", iso3: "ind",
		cities: []string{"bengaluru", "bangalore", "hyderabad", "mumbai", "bombay", "new delhi", "gurgaon", "gurugram", "noida", "pune", "chennai", "kolkata", "ahmedabad", "jaipur", "coimbatore", "kochi", "thiruvananthapuram", "indore", "chandigarh", "mysuru", "mysore", "vadodara", "nagpur"}},
	{name: "China", region: RegionAsia, iso2: "cn", iso3: "chn",
		variants: []string{"prc", "mainland china"},
		cities:   []string{"beijing", "shanghai", "shenzhen", "guangzhou", "hangzhou", "chengdu", "wuhan", "nanjing", "xi'an", "suzhou", "tianjin", "dalian", "xiamen"}},
	{name: "Hong Kong", region: RegionAsia, iso2: "hk", iso3: "hkg",
		cities: []string{"kowloon"}},
	{name: "Taiwan", region: RegionAsia, iso2: "tw", iso3: "twn",
		cities: []string{"taipei", "hsinchu", "taichung", "kaohsiung", "tainan"}},
	{name: "Japan", region: RegionAsia, iso2: "jp", iso3: "jpn",
		variants: []string{"nippon"},
		cities:   []string{"tokyo", "osaka", "kyoto", "yokohama", "nagoya", "fukuoka", "sapporo", "kobe"}},
	{name: "South Korea", region: RegionAsia, iso2: "kr", iso3: "kor",
		variants: []string{"korea", "republic of korea"},
		cities:   []string{"seoul", "busan", "incheon", "pangyo", "seongnam"}},
	{name: "Singapore", region: RegionAsia, iso2: "sg", iso3: "sgp"},
	{name: "Malaysia", region: RegionAsia, iso2: "my", iso3: "mys",
		cities: []string{"kuala lumpur", "penang", "cyberjaya", "johor bahru"}},
	{name: "Indonesia", region: RegionAsia, iso2: "id", iso3: "idn",
		cities: []string{"jakarta", "bandung", "surabaya", "denpasar", "bali"}},
	{name: "Thailand", region: RegionAsia, iso2: "th", iso3: "tha",
		cities: []string{"bangkok", "chiang mai"}},
	{name: "Vietnam", region: RegionAsia, iso2: "vn", iso3: "vnm",
		variants: []string{"viet nam"},
		cities:   []string{"ho chi minh", "hanoi", "da nang", "saigon"}},
	{name: "Philippines", region: RegionAsia, iso2: "ph", iso3: "phl",
		cities: []string{"manila", "makati", "taguig", "cebu", "quezon city", "pasig"}},
	{name: "Pakistan", region: RegionAsia, iso2: "pk", iso3: "pak",
		cities: []string{"karachi", "lahore", "islamabad"}},
	{name: "Bangladesh", region: RegionAsia, iso2: "bd", iso3: "bgd",
		cities: []string{"dhaka"}},
	{name: "Sri Lanka", region: RegionAsia, iso2: "lk", iso3: "lka",
		cities: []string{"colombo"}},
	{name: "Nepal", region: RegionAsia, iso2: "np", iso3: "npl",
		cities: []string{"kathmandu"}},
	{name: "Cambodia", region: RegionAsia, iso2: "kh", iso3: "khm",
		cities: []string{"phnom penh"}},
	{name: "Kazakhstan", region: RegionAsia, iso2: "kz", iso3: "kaz",
		cities: []string{"almaty", "astana", "nur-sultan"}},
	{name: "Uzbekistan", region: RegionAsia, iso2: "uz", iso3: "uzb",
		cities: []string{"tashkent", "samarkand"}},
	{name: "Azerbaijan", region: RegionAsia, iso2: "az", iso3: "aze",
		cities: []string{"baku"}},

	// ── Middle East ───────────────────────────────────────────────────────
	{name: "Israel", region: RegionMiddleEast, iso2: "il", iso3: "isr",
		cities: []string{"tel aviv", "jerusalem", "haifa", "herzliya", "ra'anana", "raanana", "netanya"}},
	{name: "United Arab Emirates", region: RegionMiddleEast, iso2: "ae", iso3: "are",
		variants: []string{"uae", "u.a.e"},
		cities:   []string{"dubai", "abu dhabi", "sharjah"}},
	{name: "Saudi Arabia", region: RegionMiddleEast, iso2: "sa", iso3: "sau",
		variants: []string{"ksa"},
		cities:   []string{"riyadh", "jeddah", "dammam", "neom", "khobar"}},
	{name: "Qatar", region: RegionMiddleEast, iso2: "qa", iso3: "qat",
		cities: []string{"doha"}},
	{name: "Kuwait", region: RegionMiddleEast, iso2: "kw", iso3: "kwt",
		cities: []string{"kuwait city"}},
	{name: "Bahrain", region: RegionMiddleEast, iso2: "bh", iso3: "bhr",
		cities: []string{"manama"}},
	{name: "Oman", region: RegionMiddleEast, iso2: "om", iso3: "omn",
		cities: []string{"muscat"}},
	{name: "Jordan", region: RegionMiddleEast, iso2: "jo", iso3: "jor",
		cities: []string{"amman"}},
	{name: "Lebanon", region: RegionMiddleEast, iso2: "lb", iso3: "lbn",
		cities: []string{"beirut"}},

	// ── Africa ────────────────────────────────────────────────────────────
	{name: "Egypt", region: RegionAfrica, iso2: "eg", iso3: "egy",
		cities: []string{"cairo", "alexandria", "giza"}},
	{name: "South Africa", region: RegionAfrica, iso2: "za", iso3: "zaf",
		cities: []string{"johannesburg", "cape town", "durban", "pretoria", "sandton", "centurion"}},
	{name: "Nigeria", region: RegionAfrica, iso2: "ng", iso3: "nga",
		cities: []string{"lagos", "abuja"}},
	{name: "Kenya", region: RegionAfrica, iso2: "ke", iso3: "ken",
		cities: []string{"nairobi", "mombasa"}},
	{name: "Ghana", region: RegionAfrica, iso2: "gh", iso3: "gha",
		cities: []string{"accra"}},
	{name: "Morocco", region: RegionAfrica, iso2: "ma", iso3: "mar",
		cities: []string{"casablanca", "rabat", "marrakech"}},
	{name: "Tunisia", region: RegionAfrica, iso2: "tn", iso3: "tun",
		cities: []string{"tunis"}},
	{name: "Algeria", region: RegionAfrica, iso2: "dz", iso3: "dza",
		cities: []string{"algiers"}},
	{name: "Ethiopia", region: RegionAfrica, iso2: "et", iso3: "eth",
		cities: []string{"addis ababa"}},
	{name: "Rwanda", region: RegionAfrica, iso2: "rw", iso3: "rwa",
		cities: []string{"kigali"}},
	{name: "Uganda", region: RegionAfrica, iso2: "ug", iso3: "uga",
		cities: []string{"kampala"}},
	{name: "Tanzania", region: RegionAfrica, iso2: "tz", iso3: "tza",
		cities: []string{"dar es salaam"}},
	{name: "Senegal", region: RegionAfrica, iso2: "sn", iso3: "sen",
		cities: []string{"dakar"}},
	{name: "Côte d'Ivoire", region: RegionAfrica, iso2: "ci", iso3: "civ",
		variants: []string{"ivory coast", "cote d'ivoire"},
		cities:   []string{"abidjan"}},
	{name: "Mauritius", region: RegionAfrica, iso2: "mu", iso3: "mus",
		cities: []string{"port louis"}},

	// ── Oceania ───────────────────────────────────────────────────────────
	{name: "Australia", region: RegionOceania, iso2: "au", iso3: "aus",
		cities: []string{"sydney", "melbourne", "brisbane", "perth", "adelaide", "canberra", "gold coast", "hobart"}},
	{name: "New Zealand", region: RegionOceania, iso2: "nz", iso3: "nzl",
		cities: []string{"auckland", "wellington", "christchurch"}},
}

// subdivisions maps state/province/territory markers to their country. Both
// codes (as whole location segments) and full names are listed; the resolver
// checks names first and codes as segments. These are decisive: they are what
// resolves "Paris, TX" (United States) and "London, ON" (Canada).
var subdivisionNames = map[string]string{
	// US states (+DC and territories). "georgia" is deliberately absent: it
	// collides with the country and is resolved separately.
	"alabama": "United States", "alaska": "United States", "arizona": "United States",
	"arkansas": "United States", "california": "United States", "colorado": "United States",
	"connecticut": "United States", "delaware": "United States", "florida": "United States",
	"hawaii": "United States", "idaho": "United States", "illinois": "United States",
	"indiana": "United States", "iowa": "United States", "kansas": "United States",
	"kentucky": "United States", "louisiana": "United States", "maine": "United States",
	"maryland": "United States", "massachusetts": "United States", "michigan": "United States",
	"minnesota": "United States", "mississippi": "United States", "missouri": "United States",
	"montana": "United States", "nebraska": "United States", "nevada": "United States",
	"new hampshire": "United States", "new jersey": "United States", "new mexico": "United States",
	"north carolina": "United States", "north dakota": "United States", "ohio": "United States",
	"oklahoma": "United States", "oregon": "United States", "pennsylvania": "United States",
	"rhode island": "United States", "south carolina": "United States", "south dakota": "United States",
	"tennessee": "United States", "texas": "United States", "utah": "United States",
	"vermont": "United States", "virginia": "United States", "washington": "United States",
	"west virginia": "United States", "wisconsin": "United States", "wyoming": "United States",
	"district of columbia": "United States", "puerto rico": "United States",

	// Canadian provinces.
	"ontario": "Canada", "quebec": "Canada", "québec": "Canada", "british columbia": "Canada",
	"alberta": "Canada", "manitoba": "Canada", "saskatchewan": "Canada", "nova scotia": "Canada",
	"new brunswick": "Canada", "newfoundland": "Canada", "prince edward island": "Canada",

	// Australian states/territories.
	"new south wales": "Australia", "queensland": "Australia", "tasmania": "Australia",
	"western australia": "Australia", "south australia": "Australia",
	"australian capital territory": "Australia", "northern territory": "Australia",

	// Indian states most often used in job locations.
	"karnataka": "India", "maharashtra": "India", "telangana": "India", "tamil nadu": "India",
	"haryana": "India", "uttar pradesh": "India", "kerala": "India", "west bengal": "India",
	"gujarat": "India", "rajasthan": "India", "andhra pradesh": "India", "punjab": "India",
}

// subdivisionCodes are whole-segment codes. Codes that collide with an ISO-2
// country code are omitted here and resolved by the ambiguity rules in
// location.go (e.g. "ca" California/Canada, "in" Indiana/India).
var subdivisionCodes = map[string]string{
	// US states + DC. Codes that collide with a country in countryTable
	// (ca, co, de, id, il, in, ma, md, pa, tn, ar, az) are omitted — init()
	// registers those as ambiguous instead. "mt" and "pt" are omitted as well:
	// they read as US timezone abbreviations in "Remote (MT)" strings.
	"ak": "United States", "ct": "United States", "fl": "United States", "ga": "United States",
	"hi": "United States", "ia": "United States", "ks": "United States", "ky": "United States",
	"la": "United States", "me": "United States", "mi": "United States", "mn": "United States",
	"mo": "United States", "ms": "United States", "nc": "United States", "nd": "United States",
	"ne": "United States", "nh": "United States", "nj": "United States", "nm": "United States",
	"nv": "United States", "ny": "United States", "oh": "United States", "ok": "United States",
	"or": "United States", "ri": "United States", "sc": "United States", "sd": "United States",
	"tx": "United States", "ut": "United States", "va": "United States", "vt": "United States",
	"wa": "United States", "wi": "United States", "wv": "United States", "wy": "United States",
	"dc": "United States",

	// Canadian provinces (nl/sk/pe belong to the Netherlands/Slovakia/Peru).
	"on": "Canada", "qc": "Canada", "bc": "Canada", "ab": "Canada", "mb": "Canada",
	"ns": "Canada", "nb": "Canada", "yt": "Canada", "nt": "Canada", "nu": "Canada",

	// Australian states.
	"nsw": "Australia", "qld": "Australia", "vic": "Australia", "tas": "Australia", "act": "Australia",
}

// codeBlocklist holds two-letter tokens that look like ISO codes but appear in
// job locations as something else — US timezone abbreviations, mostly
// ("Remote (PT)", "Remote US (MT)"). Locations in those countries are still
// resolved by their name or city.
var codeBlocklist = map[string]bool{"pt": true, "mt": true, "et": true, "am": true}

// regionTokens map region-level phrases to a region. Checked on word
// boundaries, longest first.
var regionTokens = map[string]string{
	"europe": RegionEurope, "european": RegionEurope, "emea": RegionEurope,
	"eurozone": RegionEurope, "schengen": RegionEurope, "eea": RegionEurope,
	"eu": RegionEurope, "e.u.": RegionEurope,
	"north america": RegionNorthAmerica, "namer": RegionNorthAmerica,
	"americas":      RegionNorthAmerica,
	"latin america": RegionLatinAmerica, "latam": RegionLatinAmerica,
	"south america": RegionLatinAmerica, "central america": RegionLatinAmerica,
	"apac": RegionAsia, "asia": RegionAsia, "asia pacific": RegionAsia,
	"southeast asia": RegionAsia, "asean": RegionAsia,
	"middle east": RegionMiddleEast, "mena": RegionMiddleEast, "gcc": RegionMiddleEast,
	"africa": RegionAfrica, "sub saharan africa": RegionAfrica,
	"oceania": RegionOceania, "australasia": RegionOceania, "anz": RegionOceania,
}

// Derived lookup tables (built in init). Phrase keys (variants, cities, region
// tokens) are normalized to the same word-joined form the matcher produces, so
// lookups are exact map hits rather than substring scans.
var (
	countryRegion   map[string]string // canonical country -> region
	countryVariants map[string]string // name variant -> canonical country
	codeCountry     map[string]string // unambiguous ISO-2/ISO-3 code -> country
	ambiguousCodes  map[string]string // ISO-2 code that is also a US state code -> country
	cityCountry     map[string]string // city -> canonical country
)

func init() {
	countryRegion = make(map[string]string, len(countryTable))
	countryVariants = map[string]string{}
	codeCountry = map[string]string{}
	ambiguousCodes = map[string]string{}
	cityCountry = map[string]string{}

	for _, c := range countryTable {
		countryRegion[c.name] = c.region
		addFirst(countryVariants, normPhrase(c.name), c.name)
		for _, v := range c.variants {
			addFirst(countryVariants, normPhrase(v), c.name)
		}
		for _, code := range []string{c.iso2, c.iso3} {
			if code == "" || codeBlocklist[code] {
				continue
			}
			if _, taken := subdivisionCodes[code]; taken {
				addFirst(ambiguousCodes, code, c.name)
				continue
			}
			addFirst(codeCountry, code, c.name)
		}
		for _, city := range c.cities {
			addFirst(cityCountry, normPhrase(city), c.name)
		}
	}
	// US state codes that collide with a country ISO-2 code: kept out of
	// subdivisionCodes (a country claims them there) and registered as
	// ambiguous, so "Cambridge, MA" resolves to the state and "Bengaluru, KA,
	// IN" to the country.
	for _, code := range []string{"ca", "co", "de", "id", "il", "in", "ma", "md", "pa", "tn", "ar", "az"} {
		if c, ok := codeCountry[code]; ok {
			ambiguousCodes[code] = c
			delete(codeCountry, code)
		}
	}
}

// addFirst inserts key->val only if key is not already present, so the first
// entry in table order wins for names shared by several countries.
func addFirst(m map[string]string, key, val string) {
	if key == "" {
		return
	}
	if _, ok := m[key]; !ok {
		m[key] = val
	}
}

// CountriesInRegion lists the canonical countries of a region, alphabetically.
func CountriesInRegion(region string) []string {
	var out []string
	for name, r := range countryRegion {
		if r == region {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
