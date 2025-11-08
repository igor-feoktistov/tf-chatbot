package utils

import (
	"fmt"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	jwt "github.com/golang-jwt/jwt/v5"
)

func MDtoHTML(md []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	return markdown.Render(doc, renderer)
}

func GetTokenClaim(tokenString string, claimName string) (tokenClaim string, err error) {
	var token *jwt.Token
	var claims jwt.MapClaims
	if token, _, err = new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{}); err != nil {
		err = fmt.Errorf("Failure to parse JWT token: %s", err)
                return
        }
	claims = token.Claims.(jwt.MapClaims)
	if tokenClaimValue, exists := claims[claimName]; exists && tokenClaimValue != nil {
		tokenClaim = tokenClaimValue.(string)
	} else {
		err = fmt.Errorf("Claim \"%s\" is not found in token claims", claimName)
	}
	return
}
