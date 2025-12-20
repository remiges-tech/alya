module auth-middleware-demo

go 1.21

require (
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/remiges-tech/alya v0.0.0
)

replace github.com/remiges-tech/alya => ../..
