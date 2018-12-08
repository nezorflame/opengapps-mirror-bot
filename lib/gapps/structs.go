package gapps

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
