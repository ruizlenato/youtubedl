package youtubedl

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

type Player struct {
	httpClient     *http.Client
	sigTimestamp   int
	sigSC          string
	nsigSC         string
	nsigName       string
	nsigCheck      string
	visitorData    string
	globalVariable *FindVariableResult
}

var (
	playerRe              = regexp.MustCompile(`(?m)player\\\/(\w+)\\/`)
	signatureTimestampRe  = regexp.MustCompile(`(?m)signatureTimestamp:(\d+),`)
	signatureSourceCodeRe = regexp.MustCompile(`(?m)function\(([A-Za-z_0-9]+)\)\{([A-Za-z_0-9]+=[A-Za-z_0-9]+\.split\((?:[^)]+)\)(.+?)\.join\((?:[^)]+)\))\}`)
	nsigCheckRe           = regexp.MustCompile(`(?m)if\(typeof (.+)\=\=\=.+\)return`)
	splitObjectRefRe      = regexp.MustCompile(`[.\[]`)

	playerCacheTTL = 5 * time.Minute
	playerCache    sync.Map
	nsigCache      sync.Map
)

type playerCacheEntry struct {
	player    *Player
	expiresAt time.Time
}

func getCachedPlayer(playerID string) (*Player, bool) {
	value, found := playerCache.Load(playerID)
	if !found {
		return nil, false
	}

	entry, ok := value.(playerCacheEntry)
	if !ok {
		playerCache.Delete(playerID)
		return nil, false
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		playerCache.Delete(playerID)
		return nil, false
	}

	return entry.player, true
}

func setCachedPlayer(playerID string, player *Player) {
	playerCache.Store(playerID, playerCacheEntry{
		player:    player,
		expiresAt: time.Now().Add(playerCacheTTL),
	})
}

func getCachedNSig(n string) (string, bool) {
	value, found := nsigCache.Load(n)
	if !found {
		return "", false
	}

	nsig, ok := value.(string)
	if !ok {
		nsigCache.Delete(n)
		return "", false
	}

	return nsig, true
}

func setCachedNSig(n string, nsig string) {
	nsigCache.Store(n, nsig)
}

// NewPlayer fetches and prepares the current YouTube player scripts and metadata.
func NewPlayer() (player *Player, err error) {
	uri, err := url.Parse(URLs.YTBase)
	if err != nil {
		return
	}

	visitorData, err := getVisitorData()
	if err != nil {
		return
	}

	player = new(Player)
	player.httpClient = &http.Client{}
	player.visitorData = visitorData

	uri.Path = path.Join(uri.Path, "/iframe_api")

	resp, err := player.httpClient.Get(uri.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if !playerRe.Match(body) {
		return
	}

	matches := playerRe.FindSubmatch(body)

	playerID := string(matches[1])

	playerc, found := getCachedPlayer(playerID)
	if found {
		return playerc, nil
	}

	playerURI, err := url.Parse(URLs.YTBase)
	if err != nil {
		return
	}

	playerURI.Path = path.Join(playerURI.Path, fmt.Sprintf("/s/player/%s/player_ias.vflset/en_US/base.js", playerID))
	req, err := http.NewRequest("GET", playerURI.String(), nil)
	if err != nil {
		return
	}
	req.Header.Add("User-Agent", RandomUserAgent())

	playerResp, err := player.httpClient.Do(req)
	if err != nil {
		return
	}
	defer playerResp.Body.Close()

	playerJSBytes, err := io.ReadAll(playerResp.Body)
	if err != nil {
		return
	}
	playerJS := string(playerJSBytes)

	player.globalVariable, err = extractGlobalVariable(playerJS)
	if err != nil {
		player.globalVariable = nil
	}

	player.sigTimestamp, err = extractSigTimestamp(playerJS)
	if err != nil {
		player.sigTimestamp = 0
	}

	if player.globalVariable != nil {
		player.sigSC, err = extractSigSourceCode(playerJS, player.globalVariable)
		if err != nil {
			player.sigSC = ""
		}
	}

	if player.globalVariable != nil {
		player.nsigName, player.nsigSC, err = extractNSigSourceCode(playerJS, player.globalVariable)
		if err != nil {
			player.nsigName = ""
			player.nsigSC = ""
		}
	}

	nsigCheck := nsigCheckRe.FindStringSubmatch(player.nsigSC)
	if len(nsigCheck) > 0 {
		player.nsigCheck = nsigCheck[1]
	}

	setCachedPlayer(playerID, player)

	return
}

func (p *Player) decipher(uri string, cipher string) (code string, err error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return
	}

	if uri == "" && p.sigSC != "" && cipher != "" {
		tmp := &url.URL{}
		tmp.RawQuery = cipher
		query := tmp.Query()

		parsedURI, err = url.Parse(query.Get("url"))
		if err != nil {
			return "", err
		}

		s := query.Get("s")
		vm := goja.New()
		vm.Set("sig", s)
		sig, err := vm.RunString(p.sigSC)
		if err != nil {
			return "", err
		}

		query2 := parsedURI.Query()
		sp := query.Get("sp")
		if sp != "" {
			query2.Set(sp, sig.String())
		} else {
			query2.Set("sig", sig.String())
		}

		parsedURI.RawQuery = query2.Encode()
	}
	query := parsedURI.Query()

	n := query.Get("n")
	if p.nsigSC != "" && n != "" {
		nsig, found := getCachedNSig(n)
		if !found {
			vm := goja.New()
			err := vm.Set(p.nsigCheck, true)
			if err != nil {
				return "", err
			}
			_, err = vm.RunString(p.nsigSC)
			if err != nil {
				return "", err
			}

			var decipher func(string) string
			err = vm.ExportTo(vm.Get(p.nsigName), &decipher)
			if err != nil {
				return "", err
			}

			nsig = decipher(n)
			setCachedNSig(n, nsig)
		}

		query.Set("n", nsig)

	}

	client := query.Get("c")
	switch client {
	case "WEB":
		query.Set("cver", Clients["WEB"].Version)
	case "MWEB":
		query.Set("cver", Clients["MWEB"].Version)
	case "WEB_REMIX":
		query.Set("cver", Clients["YTMUSIC"].Version)
	case "WEB_KIDS":
		query.Set("cver", Clients["WEB_KIDS"].Version)
	case "TVHTML5":
		query.Set("cver", Clients["TV"].Version)
	case "TVHTML5_SIMPLY_EMBEDDED_PLAYER":
		query.Set("cver", Clients["TV_EMBEDDED"].Version)
	case "WEB_EMBEDDED_PLAYER":
		query.Set("cver", Clients["WEB_EMBEDDED"].Version)
	}

	parsedURI.RawQuery = query.Encode()

	return parsedURI.String(), nil
}

func extractGlobalVariable(data string) (*FindVariableResult, error) {
	return FindVariable(string(data), FindVariableArgs{
		Includes: "-_w8_",
	})
}

func extractSigTimestamp(playerJS string) (int, error) {
	matches := signatureTimestampRe.FindStringSubmatch(playerJS)
	if len(matches) < 2 {
		return 0, fmt.Errorf("failed to extract signature timestamp")
	}

	sigTimestamp, err := strconv.Atoi(string(matches[1]))
	if err != nil {
		return 0, err
	}

	return sigTimestamp, nil
}

func extractSigSourceCode(playerJS string, g *FindVariableResult) (string, error) {
	matches := signatureSourceCodeRe.FindStringSubmatch(playerJS)

	if len(matches) == 0 && g != nil && g.Name != "" {
		escapedName := regexp.QuoteMeta(g.Name)
		lookupRegexStr := fmt.Sprintf(`function\(([A-Za-z_0-9]+)\)\{([A-Za-z_0-9]+=[A-Za-z_0-9]+\[%s\[\d+\]\]\([^)]*\)([\s\S]+?)\[%s\[\d+\]\]\([^)]*\))\}`, escapedName, escapedName)
		lookupRegex := regexp.MustCompile(lookupRegexStr)
		matches = lookupRegex.FindStringSubmatch(playerJS)
	}

	if len(matches) == 0 {
		script, err := extractSigSourceCodeByMarkers(playerJS)
		if err == nil {
			return script, nil
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("failed to extract signature decipher algorithm")
	}

	varName := string(matches[1])

	// Split on "." or "["
	splitParts := splitObjectRefRe.Split(matches[3], -1)
	var objName string

	if len(splitParts) > 0 {
		potentialObjName := strings.TrimSpace(strings.ReplaceAll(splitParts[0], ";", ""))
		objName = potentialObjName
	}

	if objName == "" {
		return "", fmt.Errorf("could not determine object name from decipher logic: %s", matches[3])
	}

	functions, err := extractObjectDefinition(playerJS, objName)
	if err != nil {
		return "", err
	}

	globalVarCode := g.Result
	if !strings.HasSuffix(strings.TrimSpace(globalVarCode), ";") {
		globalVarCode += ";"
	}

	decipherLogic := matches[2]

	return fmt.Sprintf("%s function descramble_sig(%s) { let %s={%s}; %s } descramble_sig(sig);", globalVarCode, varName, objName, functions, decipherLogic), nil
}

func extractObjectDefinition(playerJS string, objectName string) (string, error) {
	prefixes := []string{
		fmt.Sprintf("var %s={", objectName),
		fmt.Sprintf("%s={", objectName),
	}

	for _, prefix := range prefixes {
		idx := strings.Index(playerJS, prefix)
		if idx < 0 {
			continue
		}

		objSubBody := playerJS[idx+len(prefix)-1:]
		objBody, ok := cutAfterJS(objSubBody)
		if !ok {
			continue
		}

		if len(objBody) < 2 {
			return "", fmt.Errorf("object definition for '%s' is empty", objectName)
		}

		return objBody[1 : len(objBody)-1], nil
	}

	return "", fmt.Errorf("object definition for '%s' not found", objectName)
}

func extractSigSourceCodeByMarkers(playerJS string) (string, error) {
	fnName := between(playerJS, `a.set("alr","yes");c&&(c=`, "(decodeURIC")
	if fnName == "" {
		return "", fmt.Errorf("failed to locate decipher function name")
	}

	fnStart := fmt.Sprintf("%s=function(a)", fnName)
	fnIndex := strings.Index(playerJS, fnStart)
	if fnIndex < 0 {
		return "", fmt.Errorf("failed to locate decipher function body")
	}

	subBody := playerJS[fnIndex+len(fnStart):]
	fnBody, ok := cutAfterJS(subBody)
	if !ok {
		return "", fmt.Errorf("failed to parse decipher function body")
	}

	decipherFunction := fmt.Sprintf("var %s%s", fnStart, fnBody)
	manipulations := extractManipulations(playerJS, decipherFunction)

	var script strings.Builder
	if manipulations != "" {
		script.WriteString(manipulations)
		script.WriteString(";")
	}
	script.WriteString(decipherFunction)
	script.WriteString(";")
	script.WriteString(fmt.Sprintf("%s(sig);", fnName))

	return script.String(), nil
}

func extractManipulations(body string, caller string) string {
	objName := between(caller, `a=a.split("");`, ".")
	if objName == "" {
		return ""
	}

	objStart := fmt.Sprintf("var %s={", objName)
	objIndex := strings.Index(body, objStart)
	if objIndex < 0 {
		return ""
	}

	objSubBody := body[objIndex+len(objStart)-1:]
	objBody, ok := cutAfterJS(objSubBody)
	if !ok {
		return ""
	}

	return fmt.Sprintf("var %s=%s", objName, objBody)
}

func between(haystack string, left string, right string) string {
	leftIdx := strings.Index(haystack, left)
	if leftIdx < 0 {
		return ""
	}

	start := leftIdx + len(left)
	if start > len(haystack) {
		return ""
	}

	rightIdx := strings.Index(haystack[start:], right)
	if rightIdx < 0 {
		return ""
	}

	return haystack[start : start+rightIdx]
}

func cutAfterJS(mixed string) (string, bool) {
	if mixed == "" {
		return "", false
	}

	bytes := []byte(mixed)
	index := 0
	nest := 0
	var lastSignificant byte
	hasLastSignificant := false

	for nest > 0 || index == 0 {
		if index >= len(bytes) {
			return "", false
		}

		ch := bytes[index]
		switch ch {
		case '{', '[', '(':
			nest++
		case '}', ']', ')':
			nest--
		case '"', '\'', '`':
			quote := ch
			index++
			for index < len(bytes) && bytes[index] != quote {
				if bytes[index] == '\\' {
					index++
				}
				index++
			}
			if index >= len(bytes) {
				return "", false
			}
		case '/':
			if index+1 < len(bytes) && bytes[index+1] == '*' {
				index += 2
				for index+1 < len(bytes) && !(bytes[index] == '*' && bytes[index+1] == '/') {
					index++
				}
				if index+1 >= len(bytes) {
					return "", false
				}
				index++
			} else if hasLastSignificant && !((lastSignificant >= 'a' && lastSignificant <= 'z') || (lastSignificant >= 'A' && lastSignificant <= 'Z') || (lastSignificant >= '0' && lastSignificant <= '9') || lastSignificant == '_') {
				index++
				for index < len(bytes) && bytes[index] != '/' {
					if bytes[index] == '\\' {
						index++
					}
					index++
				}
				if index >= len(bytes) {
					return "", false
				}
			}
		default:
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
				lastSignificant = ch
				hasLastSignificant = true
			}
		}

		index++
	}

	if index <= 1 {
		return "", false
	}

	return mixed[:index], true
}

func extractNSigSourceCode(data string, g *FindVariableResult) (name string, code string, err error) {
	nsigFunction, err := FindFunction(string(data), FindFunctionArgs{
		Includes: fmt.Sprintf("new Date(%s", g.Name),
	})
	if err != nil {
		return
	}

	// For redundancy/the above fails:
	if nsigFunction == nil {
		nsigFunction, err = FindFunction(string(data), FindFunctionArgs{
			Includes: ".push(String.fromCharCode(",
		})
		if err != nil {
			return
		}
	}
	if nsigFunction == nil {
		nsigFunction, err = FindFunction(string(data), FindFunctionArgs{
			Includes: ".reverse().forEach(function",
		})
		if err != nil {
			return
		}
	}

	if nsigFunction != nil {
		sc := fmt.Sprintf("%s; var %s", g.Result, nsigFunction.Result)
		return nsigFunction.Name, sc, nil
	}

	nsigFunction, err = FindFunction(string(data), FindFunctionArgs{
		Includes: "-_w8_",
	})
	if err != nil {
		return
	}

	if nsigFunction == nil {
		nsigFunction, err = FindFunction(string(data), FindFunctionArgs{
			Includes: "1969",
		})
		if err != nil {
			return
		}
	}

	if nsigFunction != nil {
		return nsigFunction.Name, nsigFunction.Result, nil
	}

	return
}

type FindVariableArgs struct {
	Name     string
	Includes string
	Regexp   string
}

type FindVariableResult struct {
	Start  int
	End    int
	Name   string
	Node   ast.Node
	Result string
}

// FindVariable finds a variable assignment in JavaScript source that matches the provided criteria.
func FindVariable(source string, args FindVariableArgs) (*FindVariableResult, error) {
	var reg *regexp.Regexp
	var err error

	if args.Regexp != "" {
		reg, err = regexp.Compile(args.Regexp)
		if err != nil {
			return nil, err
		}
	}

	program, err := parser.ParseFile(nil, "", source, 0)
	if err != nil {
		return nil, fmt.Errorf("error parsing JavaScript: %v", err)
	}

	var stack []ast.Statement
	stack = append(stack, program.Body...)

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch node := current.(type) {
		case *ast.ExpressionStatement:
			switch a := node.Expression.(type) {
			case *ast.CallExpression:
				switch a := a.Callee.(type) {
				case *ast.FunctionLiteral:
					for _, v := range a.DeclarationList {
						for _, va := range v.List {
							switch ab := va.Initializer.(type) {
							case *ast.CallExpression:
								c, ok := ab.Callee.(*ast.DotExpression)
								if !ok {
									continue
								}

								id, ok := va.Target.(*ast.Identifier)
								if !ok {
									continue
								}
								code, ok := c.Left.(*ast.StringLiteral)
								if !ok {
									continue
								}

								if (args.Includes != "" && strings.Index(code.Value.String(), args.Includes) > 0) || (args.Regexp != "" && reg.MatchString(code.Value.String())) {
									result := source[va.Idx0()-1 : va.Idx1()-1]
									return &FindVariableResult{
										Start:  int(va.Idx0()),
										End:    int(va.Idx1()),
										Name:   id.Name.String(),
										Node:   va,
										Result: result,
									}, nil
								}
							}
						}
					}
				}
			}
		}
	}
	return nil, nil
}

// FindFunctionArgs defines the search parameters
type FindFunctionArgs struct {
	Name     string
	Includes string
	Regexp   string
}

// FindFunctionResult holds the search result
type FindFunctionResult struct {
	Start  int
	End    int
	Name   string
	Node   ast.Node
	Result string
}

// FindFunction finds a function assignment in JavaScript source that matches the provided criteria.
func FindFunction(source string, args FindFunctionArgs) (*FindFunctionResult, error) {
	var reg *regexp.Regexp
	var err error

	if args.Regexp != "" {
		reg, err = regexp.Compile(args.Regexp)
		if err != nil {
			return nil, err
		}
	}
	program, err := parser.ParseFile(nil, "", source, 0)
	if err != nil {
		return nil, fmt.Errorf("error parsing JavaScript: %v", err)
	}

	var stack []ast.Statement
	stack = append(stack, program.Body...)

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch node := current.(type) {
		case *ast.ExpressionStatement:
			switch a := node.Expression.(type) {
			case *ast.AssignExpression:
				id, ok := a.Left.(*ast.Identifier)
				if !ok {
					continue
				}

				_, ok = a.Right.(*ast.FunctionLiteral)
				if !ok {
					continue
				}

				code := source[a.Idx0():a.Idx1()]

				if (args.Name != "" && id.Name.String() == args.Name) ||
					(args.Includes != "" && strings.Index(code, args.Includes) > 0) || (args.Regexp != "" && reg.MatchString(code)) {
					result := source[a.Idx0()-1 : a.Idx1()-1]
					return &FindFunctionResult{
						Start:  int(a.Idx0()),
						End:    int(a.Idx1()),
						Name:   id.Name.String(),
						Node:   a,
						Result: result,
					}, nil
				}

			case *ast.CallExpression:
				switch a := a.Callee.(type) {
				case *ast.FunctionLiteral:
					stack = append(stack, a.Body.List...)
				}
			}
		}

		switch n := current.(type) {
		case *ast.BlockStatement:
			stack = append(stack, n.List...)
		}
	}

	return nil, nil
}
