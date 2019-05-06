package gapps

import "github.com/pkg/errors"

// Platform is an enum for different chip architectures
type Platform uint

// Platform consts
const (
	PlatformArm Platform = iota
	PlatformArm64
	PlatformX86
	PlatformX86_64
)

// Android is an enum for different Android versions
type Android uint

// Android consts
const (
	Android44 Android = iota
	Android50
	Android51
	Android60
	Android70
	Android71
	Android80
	Android81
	Android90
)

// Variant is an enum for different package variations
type Variant uint

// Variant consts
const (
	VariantTvstock Variant = iota
	VariantPico
	VariantNano
	VariantMicro
	VariantMini
	VariantFull
	VariantStock
	VariantSuper
	VariantAroma
)

const parsingErrText = "parsing error"

// ParsePackageParts helps to parse package info args into proper parts
func ParsePackageParts(args []string) (Platform, Android, Variant, error) {
	if len(args) != 3 {
		return 0, 0, 0, errors.Errorf("bad number of arguments: want 4, got %d", len(args))
	}

	platform, err := PlatformString(args[0])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	android, err := AndroidString(args[1])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	variant, err := VariantString(args[2])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	return platform, android, variant, nil
}
