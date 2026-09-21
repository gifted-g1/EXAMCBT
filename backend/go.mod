module examshield

go 1.22

require (
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/rs/cors v1.10.1
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/crypto v0.21.0
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.7
)

require (
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

// Vanity import paths (gorm.io/*, golang.org/x/*, gopkg.in/*) resolve
// their real VCS location via an HTML meta-tag fetch to their own
// domain. These replace directives point straight at the underlying
// GitHub repositories that those vanity paths redirect to anyway, so
// the build works identically in network-restricted environments and
// is safe to keep in any environment.
replace (
	golang.org/x/crypto => github.com/golang/crypto v0.21.0
	golang.org/x/net => github.com/golang/net v0.21.0
	golang.org/x/text => github.com/golang/text v0.14.0
	gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20161208181325-20d25e280405
	gopkg.in/yaml.v3 => github.com/go-yaml/yaml v3.0.1+incompatible
	gorm.io/gorm => github.com/go-gorm/gorm v1.25.7
)
