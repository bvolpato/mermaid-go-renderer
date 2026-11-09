package parity

// CorpusImageSize is a render.Options.ImageSize for the fixtures of the
// corpus: it gives the natural size of the images in CorpusImages, which
// are the images that the reference renderer loaded from the network. Every
// other image fails, as it does in a render with no network.
func CorpusImageSize(src string) (width, height float64, ok bool) {
	size, ok := CorpusImages[src]
	return size[0], size[1], ok
}
