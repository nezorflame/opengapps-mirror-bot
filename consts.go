package main

const (
	platformErrText = "does not belong to Platform values"
	androidErrText  = "does not belong to Android values"
	variantErrText  = "does not belong to Variant values"
	dateErrText     = "unable to parse time"
)

const (
	mirrorCmd        = "/mirror"
	helpCmd          = "/help"
	inProgressMsg    = "Looking up the package, please wait..."
	unknownErrMsg    = "Oops! Something happened. Please contact the developer."
	platformErrMsg   = "Please provide the proper platform (arm/arm64/x86/x86_64)"
	androidErrMsg    = "Please provide the proper Android version (4.4...9.0)"
	variantErrMsg    = "Please provide the proper package variant (use /help for more info)"
	dateErrMsg       = "Please provide the proper date (in the format `YYYYMMDD`)"
	mirrorErrMsg     = "Please provide the platform, Android version, package variant and date of the release (optional)."
	notFoundMsg      = "Sorry, there's no such package available. Please try another one.\nUse /help for more info."
	foundMsg         = "Package %s was found. Checking if the mirror is present..."
	mirrorMissingMsg = "There's no mirror yet, uploading..."
	mirrorMsg        = "Here's your mirror: %s\nMD5 checksum: %s\nOriginal package URL: %s"
	mirrorFailMsg    = "Sorry, I was unable to create a mirror.\nHere's your package original URL: %s\nMD5 checksum: %s"
	helpMsg          = "Please provide the platform (arm/arm64/x86/x86_64), Android version (4.4...9.0), package variant (use /help for more info) and date of the release (optional, in the format `YYYYMMDD`).\nExamples:\n  /mirror arm64 9.0 nano\n  /mirror arm 8.1 aroma 20181127"
)
