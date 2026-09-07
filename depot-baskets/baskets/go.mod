module github.com/dupakarovsky/eda-storefront/baskets

go 1.25.0

require github.com/dupakarovsky/eda-storefront/foundation v0.0.0-00010101000000-000000000000

// whenever anything asks for this module, use the local directory ../../foundation, don't go looking anywhere else"
replace github.com/dupakarovsky/eda-storefront/foundation => ../../foundation
