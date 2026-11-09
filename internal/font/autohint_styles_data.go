// Ported from skrifa 0.46.2 (generated/generated_autohint_styles.rs). A
// script converted the tables of skrifa to this file. The strings are the
// strings of skrifa, with the characters outside of ASCII written as
// escapes.
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afstyles.h, src/autofit/afscript.h, src/autofit/afranges.c,
// src/autofit/afblue.dat), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

// ahScriptClasses is SCRIPT_CLASSES of skrifa.
var ahScriptClasses = [...]ahScriptClass{
	{
		name:            "Adlam",
		group:           ahGroupDefault,
		tag:             ahTag("Adlm"),
		hintTopToBottom: false,
		stdChars:        "\U0001e90c \U0001e92e",
		blues: []ahBlueString{
			{"\U0001e90c \U0001e905 \U0001e908 \U0001e90f \U0001e914 \U0001e91a", ahBlueTop},
			{"\U0001e902 \U0001e916", ahBlueNone},
			{"\U0001e92c \U0001e92e \U0001e93b \U0001e93c \U0001e93e", ahBlueTop | ahBlueXHeight},
			{"\U0001e924 \U0001e928 \U0001e929 \U0001e92d \U0001e934 \U0001e938 \U0001e93a \U0001e940", ahBlueNone},
		},
	},
	{
		name:            "Arabic",
		group:           ahGroupDefault,
		tag:             ahTag("Arab"),
		hintTopToBottom: false,
		stdChars:        "\u0644 \u062d \u0640",
		blues: []ahBlueString{
			{"\u0627 \u0625 \u0644 \u0643 \u0637 \u0638", ahBlueTop},
			{"\u062a \u062b \u0637 \u0638 \u0643", ahBlueNone},
			{"\u0640", ahBlueNeutral},
		},
	},
	{
		name:            "Armenian",
		group:           ahGroupDefault,
		tag:             ahTag("Armn"),
		hintTopToBottom: false,
		stdChars:        "\u057d \u054d",
		blues: []ahBlueString{
			{"\u0531 \u0544 \u0552 \u054d \u0532 \u0533 \u0534 \u0555", ahBlueTop},
			{"\u0552 \u0548 \u0534 \u0543 \u0547 \u054d \u054f \u0555", ahBlueNone},
			{"\u0565 \u0567 \u056b \u0574 \u057e \u0586 \u0573", ahBlueTop},
			{"\u0561 \u0575 \u0582 \u057d \u0563 \u0577 \u0580 \u0585", ahBlueTop | ahBlueXHeight},
			{"\u0570 \u0578 \u0573 \u0561 \u0565 \u056e \u057d \u0585", ahBlueNone},
			{"\u0562 \u0568 \u056b \u056c \u0572 \u057a \u0583 \u0581", ahBlueNone},
		},
	},
	{
		name:            "Avestan",
		group:           ahGroupDefault,
		tag:             ahTag("Avst"),
		hintTopToBottom: false,
		stdChars:        "\U00010b1a",
		blues: []ahBlueString{
			{"\U00010b00 \U00010b01 \U00010b10 \U00010b1b", ahBlueTop},
			{"\U00010b00 \U00010b01", ahBlueNone},
		},
	},
	{
		name:            "Bamum",
		group:           ahGroupDefault,
		tag:             ahTag("Bamu"),
		hintTopToBottom: false,
		stdChars:        "\ua6c1 \ua6ef",
		blues: []ahBlueString{
			{"\ua6a7 \ua6a8 \ua6db \ua6c9 \ua6c1 \ua6c8 \ua6eb \ua6ef", ahBlueTop},
			{"\ua6ad \ua6b3 \ua6b6 \ua6ec \ua6a2 \ua6bd \ua6ef \ua6f2", ahBlueNone},
		},
	},
	{
		name:            "Bengali",
		group:           ahGroupDefault,
		tag:             ahTag("Beng"),
		hintTopToBottom: true,
		stdChars:        "\u09e6 \u09ea",
		blues: []ahBlueString{
			{"\u0987 \u099f \u09a0 \u09bf \u09c0 \u09c8 \u09d7", ahBlueTop},
			{"\u0993 \u098f \u09a1 \u09a4 \u09a8 \u09ac \u09b2 \u0995", ahBlueTop},
			{"\u0985 \u09a1 \u09a4 \u09a8 \u09ac \u09ad \u09b2 \u0995", ahBlueTop | ahBlueNeutral | ahBlueXHeight},
			{"\u0985 \u09a1 \u09a4 \u09a8 \u09ac \u09ad \u09b2 \u0995", ahBlueNone},
		},
	},
	{
		name:            "Buhid",
		group:           ahGroupDefault,
		tag:             ahTag("Buhd"),
		hintTopToBottom: false,
		stdChars:        "\u174b \u174f",
		blues: []ahBlueString{
			{"\u1750 \u1748", ahBlueTop},
			{"\u1745 \u174a \u174e", ahBlueTop},
			{"\u1742 \u1743 \u1749 \u174c", ahBlueTop | ahBlueXHeight},
			{"\u1740 \u1743 \u1746 \u1749 \u174b \u174f \u1751", ahBlueNone},
		},
	},
	{
		name:            "Chakma",
		group:           ahGroupDefault,
		tag:             ahTag("Cakm"),
		hintTopToBottom: false,
		stdChars:        "\U00011124 \U00011109 \U0001111b",
		blues: []ahBlueString{
			{"\U00011103 \U00011105 \U00011109 \U00011119 \U00011117", ahBlueTop},
			{"\U00011105 \U0001111b \U0001111d \U00011117 \U00011113", ahBlueNone},
			{"\U00011116\U00011133\U00011122 \U00011118\U00011133\U00011122 \U00011119\U00011133\U00011122 \U00011124\U00011133\U00011122 \U00011125\U00011133\U00011122", ahBlueNone},
		},
	},
	{
		name:            "Canadian Syllabics",
		group:           ahGroupDefault,
		tag:             ahTag("Cans"),
		hintTopToBottom: false,
		stdChars:        "\u144c \u14da",
		blues: []ahBlueString{
			{"\u15dc \u15b4 \u1401 \u14a3 \u146b \u144e \u1511 \u15f0", ahBlueTop},
			{"\u15f6 \u15b5 \u14a7 \u1403 \u144c \u148d \u1511 \u15e2", ahBlueNone},
			{"\u14d3 \u14d5 \u14c0 \u14c2 \u14c4 \u1544 \u1546 \u1623", ahBlueTop | ahBlueXHeight},
			{"\u1543 \u14c2 \u14c0 \u1542 \u14d7 \u14da \u1546 \u1623", ahBlueNone},
			{"\u142a \u1646 \u18d8 \u1422 \u14be \u18d7 \u1506", ahBlueTop},
			{"\u1646 \u15ee \u14bb \u141e \u1506 \u14a1 \u14a2 \u14d1", ahBlueNone},
		},
	},
	{
		name:            "Carian",
		group:           ahGroupDefault,
		tag:             ahTag("Cari"),
		hintTopToBottom: false,
		stdChars:        "\U000102ab \U000102c9",
		blues: []ahBlueString{
			{"\U000102a7 \U000102ab \U000102ac \U000102ad \U000102b1 \U000102ba \U000102bc \U000102bf", ahBlueTop},
			{"\U000102a3 \U000102a7 \U000102b7 \U000102c0 \U000102ab \U000102b8 \U000102c9", ahBlueNone},
		},
	},
	{
		name:            "Cherokee",
		group:           ahGroupDefault,
		tag:             ahTag("Cher"),
		hintTopToBottom: false,
		stdChars:        "\u13a4 \u13c5 \uab95",
		blues: []ahBlueString{
			{"\u13c6 \u13bb \u13ac \u13c3 \u13a4 \u13e3 \u13a6 \u13d5", ahBlueTop},
			{"\u13c6 \u13bb \u13ac \u13c3 \u13a4 \u13e3 \u13a6 \u13d5", ahBlueNone},
			{"\uab92 \uaba4 \uabb6 \uab74 \uab7e \uab97 \uab9d \uabbf", ahBlueTop},
			{"\uab96 \uab7c \uab93 \uaba0 \uabb3 \uab76 \uaba5 \uabbb", ahBlueTop | ahBlueXHeight},
			{"\uab96 \uab7c \uab93 \uaba0 \uabb3 \uab76 \uaba5 \uabbb", ahBlueNone},
			{"\u13f8 \uab90 \uab79 \uab7b", ahBlueNone},
		},
	},
	{
		name:            "Coptic",
		group:           ahGroupDefault,
		tag:             ahTag("Copt"),
		hintTopToBottom: false,
		stdChars:        "\u2c9e \u2c9f",
		blues: []ahBlueString{
			{"\u2c8c \u2c8e \u2ca0 \u2cde \u2c9e \u2c90 \u2ca4 \u2cca", ahBlueTop},
			{"\u2cd0 \u2cd8 \u2cde \u2c8e \u2c9e \u2c90 \u2cdc \u2cb0", ahBlueNone},
			{"\u2c8d \u2c8f \u2ca1 \u2cdf \u2c9f \u2c91 \u2ca5 \u2ccb", ahBlueTop | ahBlueXHeight},
			{"\u2cd1 \u2cd9 \u2cdf \u2c8f \u2c9f \u2c91 \u2cdd \u2cd2", ahBlueNone},
		},
	},
	{
		name:            "Cypriot",
		group:           ahGroupDefault,
		tag:             ahTag("Cprt"),
		hintTopToBottom: false,
		stdChars:        "\U00010805 \U00010823",
		blues: []ahBlueString{
			{"\U0001080d \U00010819 \U00010833 \U00010831 \U00010805 \U00010813 \U00010823 \U00010826", ahBlueTop},
			{"\U00010803 \U0001080a \U0001081b \U00010823 \U00010833 \U00010835 \U00010810", ahBlueNone},
			{"\U00010808 \U0001080f \U00010816", ahBlueTop},
			{"\U00010808 \U0001080f \U00010816", ahBlueNone},
		},
	},
	{
		name:            "Cyrillic",
		group:           ahGroupDefault,
		tag:             ahTag("Cyrl"),
		hintTopToBottom: false,
		stdChars:        "\u043e \u041e",
		blues: []ahBlueString{
			{"\u0411 \u0412 \u0415 \u041f \u0417 \u041e \u0421 \u042d", ahBlueTop},
			{"\u0411 \u0412 \u0415 \u0428 \u0417 \u041e \u0421 \u042d", ahBlueNone},
			{"\u0445 \u043f \u043d \u0448 \u0435 \u0437 \u043e \u0441", ahBlueTop | ahBlueXHeight},
			{"\u0445 \u043f \u043d \u0448 \u0435 \u0437 \u043e \u0441", ahBlueNone},
			{"\u0440 \u0443 \u0444", ahBlueNone},
		},
	},
	{
		name:            "Devanagari",
		group:           ahGroupDefault,
		tag:             ahTag("Deva"),
		hintTopToBottom: true,
		stdChars:        "\u0920 \u0935 \u091f",
		blues: []ahBlueString{
			{"\u0908 \u0910 \u0913 \u0914 \u093f \u0940 \u094b \u094c", ahBlueTop},
			{"\u0915 \u092e \u0905 \u0906 \u0925 \u0927 \u092d \u0936", ahBlueTop},
			{"\u0915 \u0928 \u092e \u0909 \u091b \u091f \u0920 \u0921", ahBlueTop | ahBlueNeutral | ahBlueXHeight},
			{"\u0915 \u0928 \u092e \u0909 \u091b \u091f \u0920 \u0921", ahBlueNone},
			{"\u0941 \u0943", ahBlueNone},
		},
	},
	{
		name:            "Deseret",
		group:           ahGroupDefault,
		tag:             ahTag("Dsrt"),
		hintTopToBottom: false,
		stdChars:        "\U00010404 \U0001042c",
		blues: []ahBlueString{
			{"\U00010402 \U00010404 \U0001040b \U00010417 \U00010411", ahBlueTop},
			{"\U00010400 \U00010402 \U00010404 \U00010417 \U0001041b", ahBlueNone},
			{"\U0001042a \U0001042c \U00010433 \U0001043f \U00010439", ahBlueTop | ahBlueXHeight},
			{"\U00010428 \U0001042a \U0001042c \U0001043f \U00010443", ahBlueNone},
		},
	},
	{
		name:            "Ethiopic",
		group:           ahGroupDefault,
		tag:             ahTag("Ethi"),
		hintTopToBottom: false,
		stdChars:        "\u12d0",
		blues: []ahBlueString{
			{"\u1200 \u1203 \u12d8 \u1350 \u121b \u1260 \u12cb \u12d0", ahBlueTop},
			{"\u1208 \u1210 \u1260 \u12d8 \u1200 \u122a \u12d0 \u1328", ahBlueNone},
		},
	},
	{
		name:            "Georgian (Mkhedruli)",
		group:           ahGroupDefault,
		tag:             ahTag("Geor"),
		hintTopToBottom: false,
		stdChars:        "\u10d8 \u10d4 \u10d0 \u1cbf",
		blues: []ahBlueString{
			{"\u10d2 \u10d3 \u10d4 \u10d5 \u10d7 \u10d8 \u10dd \u10e6", ahBlueTop | ahBlueXHeight},
			{"\u10d0 \u10d6 \u10db \u10e1 \u10e8 \u10eb \u10ee \u10de", ahBlueNone},
			{"\u10e1 \u10ee \u10e5 \u10d6 \u10db \u10e8 \u10e9 \u10ec", ahBlueTop},
			{"\u10d4 \u10d5 \u10df \u10e2 \u10e3 \u10e4 \u10e5 \u10e7", ahBlueNone},
			{"\u1c9c \u1c9f \u1cb3 \u1cb8 \u1c92 \u1c94 \u1c9d \u1cb4", ahBlueTop},
			{"\u1c98 \u1cb2 \u1c9d \u1ca9 \u1c9b \u1ca8 \u1caf \u1cbd", ahBlueNone},
		},
	},
	{
		name:            "Georgian (Khutsuri)",
		group:           ahGroupDefault,
		tag:             ahTag("Geok"),
		hintTopToBottom: false,
		stdChars:        "\u10b6 \u10b1 \u2d19",
		blues: []ahBlueString{
			{"\u10b1 \u10a7 \u10b9 \u10bc \u10a4 \u10a5 \u10b3 \u10ba", ahBlueTop},
			{"\u10a4 \u10a5 \u10a7 \u10a8 \u10a6 \u10b1 \u10aa \u10ab", ahBlueNone},
			{"\u2d01 \u2d17 \u2d02 \u2d04 \u2d05 \u2d07 \u2d14 \u2d16", ahBlueTop | ahBlueXHeight},
			{"\u2d08 \u2d0c \u2d16 \u2d0e \u2d03 \u2d06 \u2d0b \u2d22", ahBlueNone},
			{"\u2d10 \u2d11 \u2d13 \u2d15 \u2d19 \u2d1b \u2d21 \u2d23", ahBlueTop},
			{"\u2d04 \u2d05 \u2d14 \u2d15 \u2d01 \u2d02 \u2d18 \u2d1d", ahBlueNone},
		},
	},
	{
		name:            "Glagolitic",
		group:           ahGroupDefault,
		tag:             ahTag("Glag"),
		hintTopToBottom: false,
		stdChars:        "\u2c15 \u2c45",
		blues: []ahBlueString{
			{"\u2c05 \u2c14 \u2c2a \u2c04 \u2c02 \u2c0a \u2c2b \u2c0b", ahBlueTop},
			{"\u2c05 \u2c04 \u2c02 \u2c2a \u2c1e \u2c21 \u2c0a \u2c14", ahBlueNone},
			{"\u2c35 \u2c44 \u2c5a \u2c34 \u2c32 \u2c3a \u2c5b \u2c3b", ahBlueTop | ahBlueXHeight},
			{"\u2c35 \u2c34 \u2c32 \u2c5a \u2c4e \u2c51 \u2c3a \u2c44", ahBlueNone},
		},
	},
	{
		name:            "Gothic",
		group:           ahGroupDefault,
		tag:             ahTag("Goth"),
		hintTopToBottom: true,
		stdChars:        "\U00010334 \U0001033e \U00010343",
		blues: []ahBlueString{
			{"\U00010332 \U00010336 \U00010340 \U00010344 \U00010334 \U00010343 \U00010348 \U0001033e", ahBlueTop},
			{"\U00010336 \U00010334 \U00010343 \U00010348", ahBlueNone},
		},
	},
	{
		name:            "Greek",
		group:           ahGroupDefault,
		tag:             ahTag("Grek"),
		hintTopToBottom: false,
		stdChars:        "\u03bf \u039f",
		blues: []ahBlueString{
			{"\u0393 \u0392 \u0395 \u0396 \u0398 \u039f \u03a9", ahBlueTop},
			{"\u0392 \u0394 \u0396 \u039e \u0398 \u039f", ahBlueNone},
			{"\u03b2 \u03b8 \u03b4 \u03b6 \u03bb \u03be", ahBlueTop},
			{"\u03b1 \u03b5 \u03b9 \u03bf \u03c0 \u03c3 \u03c4 \u03c9", ahBlueTop | ahBlueXHeight},
			{"\u03b1 \u03b5 \u03b9 \u03bf \u03c0 \u03c3 \u03c4 \u03c9", ahBlueNone},
			{"\u03b2 \u03b3 \u03b7 \u03bc \u03c1 \u03c6 \u03c7 \u03c8", ahBlueNone},
		},
	},
	{
		name:            "Gujarati",
		group:           ahGroupDefault,
		tag:             ahTag("Gujr"),
		hintTopToBottom: false,
		stdChars:        "\u0a9f \u0ae6",
		blues: []ahBlueString{
			{"\u0aa4 \u0aa8 \u0a8b \u0a8c \u0a9b \u0a9f \u0ab0 \u0ae6", ahBlueTop | ahBlueXHeight},
			{"\u0a96 \u0a97 \u0a98 \u0a9e \u0a87 \u0a88 \u0aa0 \u0a9c", ahBlueNone},
			{"\u0a88 \u0a8a \u0abf \u0ac0 \u0ab2\u0ac0 \u0ab6\u0acd\u0a9a\u0abf \u0a9c\u0abf \u0ab8\u0ac0", ahBlueTop},
			{"\u0ac1 \u0ac3 \u0ac4 \u0a96\u0ac1 \u0a9b\u0ac3 \u0a9b\u0ac4", ahBlueNone},
			{"\u0ae6 \u0ae7 \u0ae8 \u0ae9 \u0aed", ahBlueTop},
		},
	},
	{
		name:            "Gurmukhi",
		group:           ahGroupDefault,
		tag:             ahTag("Guru"),
		hintTopToBottom: true,
		stdChars:        "\u0a20 \u0a30 \u0a66",
		blues: []ahBlueString{
			{"\u0a07 \u0a08 \u0a09 \u0a0f \u0a13 \u0a73 \u0a3f \u0a40", ahBlueTop},
			{"\u0a15 \u0a17 \u0a19 \u0a1a \u0a1c \u0a24 \u0a27 \u0a38", ahBlueTop},
			{"\u0a15 \u0a17 \u0a19 \u0a1a \u0a1c \u0a24 \u0a27 \u0a38", ahBlueTop | ahBlueNeutral | ahBlueXHeight},
			{"\u0a05 \u0a0f \u0a13 \u0a17 \u0a1c \u0a20 \u0a30 \u0a38", ahBlueNone},
			{"\u0a66 \u0a67 \u0a68 \u0a69 \u0a6d", ahBlueTop},
		},
	},
	{
		name:            "Hebrew",
		group:           ahGroupDefault,
		tag:             ahTag("Hebr"),
		hintTopToBottom: false,
		stdChars:        "\u05dd",
		blues: []ahBlueString{
			{"\u05d1 \u05d3 \u05d4 \u05d7 \u05da \u05db \u05dd \u05e1", ahBlueTop | ahBlueLong},
			{"\u05d1 \u05d8 \u05db \u05dd \u05e1 \u05e6", ahBlueNone},
			{"\u05e7 \u05da \u05df \u05e3 \u05e5", ahBlueNone},
		},
	},
	{
		name:            "Nyiakeng Puachue Hmong",
		group:           ahGroupDefault,
		tag:             ahTag("Hmnp"),
		hintTopToBottom: false,
		stdChars:        "\U0001e128",
		blues: []ahBlueString{
			{"\U0001e100 \U0001e101 \U0001e108 \U0001e111 \U0001e127 \U0001e12c", ahBlueTop},
			{"\U0001e101 \U0001e11c \U0001e120 \U0001e121 \U0001e124 \U0001e142", ahBlueNone},
		},
	},
	{
		name:            "Kayah Li",
		group:           ahGroupDefault,
		tag:             ahTag("Kali"),
		hintTopToBottom: false,
		stdChars:        "\ua90d \ua900",
		blues: []ahBlueString{
			{"\ua905 \ua90f \ua901 \ua90b \ua900 \ua90d", ahBlueTop | ahBlueXHeight},
			{"\ua908 \ua918 \ua900 \ua90d \ua922", ahBlueNone},
			{"\ua916 \ua921", ahBlueTop},
			{"\ua911 \ua91c \ua91e", ahBlueNone},
			{"\ua911\ua92c \ua91c\ua92d \ua914\ua92c", ahBlueNone},
		},
	},
	{
		name:            "Khmer",
		group:           ahGroupDefault,
		tag:             ahTag("Khmr"),
		hintTopToBottom: false,
		stdChars:        "\u17e0",
		blues: []ahBlueString{
			{"\u1781 \u1791 \u1793 \u17a7 \u17a9 \u17b6", ahBlueTop | ahBlueXHeight},
			{"\u1780\u17d2\u1780 \u1780\u17d2\u1781 \u1780\u17d2\u1782 \u1780\u17d2\u1790", ahBlueSubTop},
			{"\u1781 \u1783 \u1785 \u178b \u1794 \u1798 \u1799 \u17b2", ahBlueNone},
			{"\u178f\u17d2\u179a \u179a\u17c0 \u17b2\u17d2\u1799 \u17a2\u17bf", ahBlueNone},
			{"\u1793\u17d2\u178f\u17d2\u179a\u17c3 \u1784\u17d2\u1781\u17d2\u1799 \u1780\u17d2\u1794\u17c0 \u1785\u17d2\u179a\u17c0 \u1793\u17d2\u178f\u17bf \u179b\u17d2\u1794\u17bf", ahBlueNone},
		},
	},
	{
		name:            "Khmer Symbols",
		group:           ahGroupDefault,
		tag:             ahTag("Khms"),
		hintTopToBottom: false,
		stdChars:        "\u19e1 \u19ea",
		blues: []ahBlueString{
			{"\u19e0 \u19e1", ahBlueTop | ahBlueXHeight},
			{"\u19f6 \u19f9", ahBlueNone},
		},
	},
	{
		name:            "Kannada",
		group:           ahGroupDefault,
		tag:             ahTag("Knda"),
		hintTopToBottom: false,
		stdChars:        "\u0ce6 \u0cac",
		blues: []ahBlueString{
			{"\u0c87 \u0c8a \u0c90 \u0ca3 \u0cb8\u0cbe \u0ca8\u0cbe \u0ca6\u0cbe \u0cb0\u0cbe", ahBlueTop},
			{"\u0c85 \u0c89 \u0c8e \u0cb2 \u0ce6 \u0ce8 \u0cec \u0ced", ahBlueNone},
		},
	},
	{
		name:            "Lao",
		group:           ahGroupDefault,
		tag:             ahTag("Laoo"),
		hintTopToBottom: false,
		stdChars:        "\u0ed0",
		blues: []ahBlueString{
			{"\u0eb2 \u0e94 \u0ead \u0ea1 \u0ea5 \u0ea7 \u0ea3 \u0e87", ahBlueTop | ahBlueXHeight},
			{"\u0eb2 \u0ead \u0e9a \u0e8d \u0ea3 \u0eae \u0ea7 \u0ea2", ahBlueNone},
			{"\u0e9b \u0ea2 \u0e9f \u0e9d", ahBlueTop},
			{"\u0ec2 \u0ec4 \u0ec3", ahBlueTop},
			{"\u0e87 \u0e8a \u0e96 \u0ebd \u0ec6 \u0eaf", ahBlueNone},
		},
	},
	{
		name:            "Latin",
		group:           ahGroupDefault,
		tag:             ahTag("Latn"),
		hintTopToBottom: false,
		stdChars:        "o O 0",
		blues: []ahBlueString{
			{"T H E Z O C Q S", ahBlueTop},
			{"H E Z L O C U S", ahBlueNone},
			{"f i j k d b h", ahBlueTop},
			{"u v x z o e s c", ahBlueTop | ahBlueXHeight},
			{"n r x z o e s c", ahBlueNone},
			{"p q g j y", ahBlueNone},
		},
	},
	{
		name:            "Latin Subscript Fallback",
		group:           ahGroupDefault,
		tag:             ahTag("Latb"),
		hintTopToBottom: false,
		stdChars:        "\u2092 \u2080",
		blues: []ahBlueString{
			{"\u2080 \u2083 \u2085 \u2087 \u2088", ahBlueTop},
			{"\u2080 \u2081 \u2082 \u2083 \u2088", ahBlueNone},
			{"\u1d62 \u2c7c \u2095 \u2096 \u2097", ahBlueTop},
			{"\u2090 \u2091 \u2092 \u2093 \u2099 \u209b \u1d65 \u1d64 \u1d63", ahBlueTop | ahBlueXHeight},
			{"\u2090 \u2091 \u2092 \u2093 \u2099 \u209b \u1d65 \u1d64 \u1d63", ahBlueNone},
			{"\u1d66 \u1d67 \u1d68 \u1d69 \u209a", ahBlueNone},
		},
	},
	{
		name:            "Latin Superscript Fallback",
		group:           ahGroupDefault,
		tag:             ahTag("Latp"),
		hintTopToBottom: false,
		stdChars:        "\u1d52 \u1d3c \u2070",
		blues: []ahBlueString{
			{"\u2070 \u00b3 \u2075 \u2077 \u1d40 \u1d34 \u1d31 \u1d3c", ahBlueTop},
			{"\u2070 \u00b9 \u00b2 \u00b3 \u1d31 \u1d38 \u1d3c \u1d41", ahBlueNone},
			{"\u1d47 \u1d48 \u1d4f \u02b0 \u02b2 \u1da0 \u2071", ahBlueTop},
			{"\u1d49 \u1d52 \u02b3 \u02e2 \u02e3 \u1d9c \u1dbb", ahBlueTop | ahBlueXHeight},
			{"\u1d49 \u1d52 \u02b3 \u02e2 \u02e3 \u1d9c \u1dbb", ahBlueNone},
			{"\u1d56 \u02b8 \u1d4d", ahBlueNone},
		},
	},
	{
		name:            "Lisu",
		group:           ahGroupDefault,
		tag:             ahTag("Lisu"),
		hintTopToBottom: false,
		stdChars:        "\ua4f3",
		blues: []ahBlueString{
			{"\ua4e1 \ua4e7 \ua4f1 \ua4f6 \ua4e9 \ua4da \ua4f5 \ua4f3", ahBlueTop},
			{"\ua4d5 \ua4dc \ua4de \ua4e1 \ua4db \ua4e2 \ua4f3 \ua4f4", ahBlueNone},
		},
	},
	{
		name:            "Malayalam",
		group:           ahGroupDefault,
		tag:             ahTag("Mlym"),
		hintTopToBottom: false,
		stdChars:        "\u0d20 \u0d31",
		blues: []ahBlueString{
			{"\u0d12 \u0d1f \u0d20 \u0d31 \u0d1a \u0d2a \u0d1a\u0d4d\u0d1a \u0d2a\u0d4d\u0d2a", ahBlueTop},
			{"\u0d1f \u0d20 \u0d27 \u0d36 \u0d18 \u0d1a \u0d25 \u0d32", ahBlueNone},
		},
	},
	{
		name:            "Medefaidrin",
		group:           ahGroupDefault,
		tag:             ahTag("Medf"),
		hintTopToBottom: false,
		stdChars:        "\U00016e61 \U00016e5b \U00016e6f",
		blues: []ahBlueString{
			{"\U00016e40 \U00016e41 \U00016e42 \U00016e43 \U00016e4f \U00016e5a \U00016e5f", ahBlueTop},
			{"\U00016e40 \U00016e41 \U00016e42 \U00016e43 \U00016e4f \U00016e5a \U00016e52 \U00016e53", ahBlueNone},
			{"\U00016e64 \U00016e6c \U00016e67 \U00016e74 \U00016e76 \U00016e7e", ahBlueTop},
			{"\U00016e60 \U00016e61 \U00016e62 \U00016e79 \U00016e73 \U00016e6e", ahBlueTop | ahBlueXHeight},
			{"\U00016e60 \U00016e61 \U00016e62 \U00016e73 \U00016e6d \U00016e7d", ahBlueNone},
			{"\U00016e65 \U00016e68 \U00016e69", ahBlueNone},
			{"\U00016e80 \U00016e85 \U00016e88 \U00016e84 \U00016e8d", ahBlueTop},
		},
	},
	{
		name:            "Mongolian",
		group:           ahGroupDefault,
		tag:             ahTag("Mong"),
		hintTopToBottom: true,
		stdChars:        "\u1842 \u182a",
		blues: []ahBlueString{
			{"\u1833 \u1834 \u1836 \u183d \u1842 \u184a \\u{200d}\u1861\\u{200d} \\u{200d}\u1873\\u{200d}", ahBlueTop},
			{"\u1843", ahBlueNone},
		},
	},
	{
		name:            "Myanmar",
		group:           ahGroupDefault,
		tag:             ahTag("Mymr"),
		hintTopToBottom: false,
		stdChars:        "\u101d \u1004 \u1002",
		blues: []ahBlueString{
			{"\u1001 \u1002 \u1004 \u1012 \u101d \u1065 \u104a \u104b", ahBlueTop | ahBlueXHeight},
			{"\u1004 \u100e \u1012 \u1015 \u1017 \u101d \u104a \u104b", ahBlueNone},
			{"\u1029 \u103c \u104d \u104f \u1046 \u102b \u102d", ahBlueTop},
			{"\u1009 \u100a \u1025 \u1029 \u1028 \u1042 \u1045 \u1049", ahBlueNone},
		},
	},
	{
		name:            "N'Ko",
		group:           ahGroupDefault,
		tag:             ahTag("Nkoo"),
		hintTopToBottom: false,
		stdChars:        "\u07cb \u07c0",
		blues: []ahBlueString{
			{"\u07d0 \u07c9 \u07d2 \u07df \u07d6 \u07dc \u07e0 \u07e5", ahBlueTop},
			{"\u07c0 \u07d8 \u07e1 \u07e0 \u07e5", ahBlueNone},
			{"\u07cf \u07db \u07cb", ahBlueTop | ahBlueXHeight},
			{"\u07ce \u07cf \u07db \u07cb", ahBlueNone},
		},
	},
	{
		name:            "no script",
		group:           ahGroupDefault,
		tag:             ahTag("None"),
		hintTopToBottom: false,
		stdChars:        "",
	},
	{
		name:            "Ol Chiki",
		group:           ahGroupDefault,
		tag:             ahTag("Olck"),
		hintTopToBottom: false,
		stdChars:        "\u1c5b",
		blues: []ahBlueString{
			{"\u1c5b \u1c5c \u1c5d \u1c61 \u1c62 \u1c65", ahBlueTop},
			{"\u1c5b \u1c5c \u1c5d \u1c61 \u1c62 \u1c65", ahBlueNone},
		},
	},
	{
		name:            "Old Turkic",
		group:           ahGroupDefault,
		tag:             ahTag("Orkh"),
		hintTopToBottom: false,
		stdChars:        "\U00010c17",
		blues: []ahBlueString{
			{"\U00010c17 \U00010c18 \U00010c27", ahBlueTop},
			{"\U00010c09 \U00010c17 \U00010c26 \U00010c27", ahBlueNone},
		},
	},
	{
		name:            "Osage",
		group:           ahGroupDefault,
		tag:             ahTag("Osge"),
		hintTopToBottom: false,
		stdChars:        "\U000104c2 \U000104ea",
		blues: []ahBlueString{
			{"\U000104be \U000104cd \U000104d2 \U000104d3 \U000104bb \U000104c2 \U000104b5 \U000104c6", ahBlueTop},
			{"\U000104b0 \U000104cd \U000104c2 \U000104bf \U000104ce \U000104b9", ahBlueNone},
			{"\U000104bc \U000104bd \U000104be", ahBlueNone},
			{"\U000104f5 \U000104f6 \U000104fa \U000104fb \U000104dd \U000104e3 \U000104ea \U000104ee", ahBlueTop | ahBlueXHeight},
			{"\U000104d8 \U000104da \U000104e3 \U000104f5 \U000104e1 \U000104e7 \U000104ea \U000104f6", ahBlueNone},
			{"\U000104e4 \U000104e6 \U000104f8 \U000104f9 \U000104db", ahBlueTop},
			{"\U000104e4 \U000104e5 \U000104e6", ahBlueNone},
		},
	},
	{
		name:            "Osmanya",
		group:           ahGroupDefault,
		tag:             ahTag("Osma"),
		hintTopToBottom: false,
		stdChars:        "\U00010486 \U000104a0",
		blues: []ahBlueString{
			{"\U00010486 \U00010489 \U00010490 \U00010492 \U00010498 \U0001049b \U000104a0 \U000104a3", ahBlueTop},
			{"\U00010480 \U00010482 \U00010486 \U00010488 \U0001048a \U00010492 \U000104a0 \U000104a9", ahBlueNone},
		},
	},
	{
		name:            "Hanifi Rohingya",
		group:           ahGroupDefault,
		tag:             ahTag("Rohg"),
		hintTopToBottom: false,
		stdChars:        "\U00010d30",
		blues: []ahBlueString{
			{"\U00010d03 \U00010d00 \U00010d06 \U00010d16 \U00010d15", ahBlueTop},
			{"\U00010d14 \U00010d16 \U00010d15 \U00010d11 \U00010d10", ahBlueNone},
			{"\u0640", ahBlueNeutral},
		},
	},
	{
		name:            "Saurashtra",
		group:           ahGroupDefault,
		tag:             ahTag("Saur"),
		hintTopToBottom: false,
		stdChars:        "\ua89d \ua8d0",
		blues: []ahBlueString{
			{"\ua89c \ua89e \ua8b3 \ua882 \ua896 \ua892 \ua89d \ua89b", ahBlueTop},
			{"\ua882 \ua8a8 \ua8ba \ua8a4 \ua88e", ahBlueNone},
		},
	},
	{
		name:            "Shavian",
		group:           ahGroupDefault,
		tag:             ahTag("Shaw"),
		hintTopToBottom: false,
		stdChars:        "\U00010474",
		blues: []ahBlueString{
			{"\U00010455 \U00010459", ahBlueTop},
			{"\U00010454 \U00010456 \U00010457 \U00010479 \U0001047b", ahBlueNone},
			{"\U0001045f \U00010463", ahBlueNone},
			{"\U00010471 \U00010472 \U00010473 \U00010474 \U00010478 \U0001047a \U0001047c", ahBlueTop | ahBlueXHeight},
			{"\U00010474 \U0001047b \U00010479", ahBlueNone},
		},
	},
	{
		name:            "Sinhala",
		group:           ahGroupDefault,
		tag:             ahTag("Sinh"),
		hintTopToBottom: false,
		stdChars:        "\u0da7",
		blues: []ahBlueString{
			{"\u0d89 \u0d9a \u0d9d \u0db3 \u0db4 \u0dba \u0dbd \u0dc6", ahBlueTop},
			{"\u0d91 \u0d94 \u0d9d \u0da2 \u0da7 \u0dae \u0db0 \u0dbb", ahBlueNone},
			{"\u0daf \u0db3 \u0d8b \u0dbd \u0dad\u0dd6 \u0dad\u0dd4 \u0db6\u0dd4 \u0daf\u0dd4", ahBlueNone},
		},
	},
	{
		name:            "Sundanese",
		group:           ahGroupDefault,
		tag:             ahTag("Sund"),
		hintTopToBottom: false,
		stdChars:        "\u1bb0",
		blues: []ahBlueString{
			{"\u1b8b \u1b9e \u1bae \u1bbd \u1bb0 \u1b88", ahBlueTop},
			{"\u1b84 \u1b94 \u1b95 \u1b97 \u1bb0 \u1b86 \u1b88 \u1b89", ahBlueNone},
			{"\u1bbc \u1cc4", ahBlueNone},
		},
	},
	{
		name:            "Tamil",
		group:           ahGroupDefault,
		tag:             ahTag("Taml"),
		hintTopToBottom: false,
		stdChars:        "\u0be6",
		blues: []ahBlueString{
			{"\u0b89 \u0b92 \u0b93 \u0bb1 \u0b88 \u0b95 \u0b99 \u0b9a", ahBlueTop},
			{"\u0b95 \u0b9a \u0bb2 \u0bb6 \u0b89 \u0b99 \u0b9f \u0baa", ahBlueNone},
		},
	},
	{
		name:            "Tai Viet",
		group:           ahGroupDefault,
		tag:             ahTag("Tavt"),
		hintTopToBottom: false,
		stdChars:        "\uaa92 \uaaab",
		blues: []ahBlueString{
			{"\uaa86 \uaa94 \uaa92 \uaa96 \uaaab", ahBlueTop},
			{"\uaa89 \uaaab \uaaae", ahBlueNone},
		},
	},
	{
		name:            "Telugu",
		group:           ahGroupDefault,
		tag:             ahTag("Telu"),
		hintTopToBottom: false,
		stdChars:        "\u0c66 \u0c67",
		blues: []ahBlueString{
			{"\u0c07 \u0c0c \u0c19 \u0c1e \u0c23 \u0c31 \u0c6f", ahBlueTop},
			{"\u0c05 \u0c15 \u0c1a \u0c30 \u0c3d \u0c68 \u0c6c", ahBlueNone},
		},
	},
	{
		name:            "Tifinagh",
		group:           ahGroupDefault,
		tag:             ahTag("Tfng"),
		hintTopToBottom: false,
		stdChars:        "\u2d54",
		blues: []ahBlueString{
			{"\u2d54 \u2d59 \u2d5b \u2d5e \u2d35 \u2d3c \u2d39 \u2d4e", ahBlueTop},
			{"\u2d54 \u2d59 \u2d5b \u2d5e \u2d35 \u2d3c \u2d39 \u2d4e", ahBlueNone},
		},
	},
	{
		name:            "Thai",
		group:           ahGroupDefault,
		tag:             ahTag("Thai"),
		hintTopToBottom: false,
		stdChars:        "\u0e32 \u0e45 \u0e50",
		blues: []ahBlueString{
			{"\u0e1a \u0e40 \u0e41 \u0e2d \u0e01 \u0e32", ahBlueTop | ahBlueXHeight},
			{"\u0e1a \u0e1b \u0e29 \u0e2f \u0e2d \u0e22 \u0e2e", ahBlueNone},
			{"\u0e1b \u0e1d \u0e1f", ahBlueTop},
			{"\u0e42 \u0e43 \u0e44", ahBlueTop},
			{"\u0e0e \u0e0f \u0e24 \u0e26", ahBlueNone},
			{"\u0e0d \u0e10", ahBlueNone},
			{"\u0e50 \u0e51 \u0e53", ahBlueNone},
		},
	},
	{
		name:            "Vai",
		group:           ahGroupDefault,
		tag:             ahTag("Vaii"),
		hintTopToBottom: false,
		stdChars:        "\ua613 \ua59c \ua5b4",
		blues: []ahBlueString{
			{"\ua5cd \ua616 \ua619 \ua61c \ua59c \ua59d \ua505 \ua562", ahBlueTop},
			{"\ua5cd \ua616 \ua619 \ua5de \ua505 \ua562 \ua59c \ua506", ahBlueNone},
		},
	},
	{
		name:            "Limbu",
		group:           ahGroupIndic,
		tag:             ahTag("Limb"),
		hintTopToBottom: false,
		stdChars:        "o",
	},
	{
		name:            "Oriya",
		group:           ahGroupIndic,
		tag:             ahTag("Orya"),
		hintTopToBottom: false,
		stdChars:        "o",
	},
	{
		name:            "Syloti Nagri",
		group:           ahGroupIndic,
		tag:             ahTag("Sylo"),
		hintTopToBottom: false,
		stdChars:        "o",
	},
	{
		name:            "Tibetan",
		group:           ahGroupIndic,
		tag:             ahTag("Tibt"),
		hintTopToBottom: false,
		stdChars:        "o",
	},
	{
		name:            "CJKV ideographs",
		group:           ahGroupCjk,
		tag:             ahTag("Hani"),
		hintTopToBottom: false,
		stdChars:        "\u7530 \u56d7",
		blues: []ahBlueString{
			{"\u4ed6 \u4eec \u4f60 \u4f86 \u5011 \u5230 \u548c \u5730 \u5bf9 \u5c0d \u5c31 \u5e2d \u6211 \u65f6 \u6642 \u6703 \u6765 \u70ba \u80fd \u8230 \u8aaa \u8bf4 \u8fd9 \u9019 \u9f4a | \u519b \u540c \u5df2 \u613f \u65e2 \u661f \u662f \u666f \u6c11 \u7167 \u73b0 \u73fe \u7406 \u7528 \u7f6e \u8981 \u8ecd \u90a3 \u914d \u91cc \u958b \u96f7 \u9732 \u9762 \u987e", ahBlueTop},
			{"\u4e2a \u4e3a \u4eba \u4ed6 \u4ee5 \u4eec \u4f60 \u4f86 \u500b \u5011 \u5230 \u548c \u5927 \u5bf9 \u5c0d \u5c31 \u6211 \u65f6 \u6642 \u6709 \u6765 \u70ba \u8981 \u8aaa \u8bf4 | \u4e3b \u4e9b \u56e0 \u5b83 \u60f3 \u610f \u7406 \u751f \u7576 \u770b \u7740 \u7f6e \u8005 \u81ea \u8457 \u88e1 \u8fc7 \u8fd8 \u8fdb \u9032 \u904e \u9053 \u9084 \u91cc \u9762", ahBlueNone},
			{" \u4e9b \u4eec \u4f60 \u4f86 \u5011 \u5230 \u548c \u5730 \u5979 \u5c06 \u5c07 \u5c31 \u5e74 \u5f97 \u60c5 \u6700 \u6837 \u6a23 \u7406 \u80fd \u8aaa \u8bf4 \u8fd9 \u9019 \u901a | \u5373 \u5417 \u5427 \u542c \u5462 \u54c1 \u54cd \u55ce \u5e08 \u5e2b \u6536 \u65ad \u65b7 \u660e \u773c \u9593 \u95f4 \u9645 \u9648 \u9650 \u9664 \u9673 \u968f \u969b \u96a8", ahBlueHorizontal},
			{"\u4e8b \u524d \u5b78 \u5c06 \u5c07 \u60c5 \u60f3 \u6216 \u653f \u65af \u65b0 \u6837 \u6a23 \u6c11 \u6c92 \u6ca1 \u7136 \u7279 \u73b0 \u73fe \u7403 \u7b2c \u7d93 \u8c01 \u8d77 | \u4f8b \u5225 \u522b \u5236 \u52a8 \u52d5 \u5417 \u55ce \u589e \u6307 \u660e \u671d \u671f \u6784 \u7269 \u786e \u79cd \u8abf \u8c03 \u8cbb \u8d39 \u90a3 \u90fd \u9593 \u95f4", ahBlueHorizontal | ahBlueRight},
		},
	},
}

// Indices of the script classes.
const (
	ahScriptAdlm = 0
	ahScriptArab = 1
	ahScriptArmn = 2
	ahScriptAvst = 3
	ahScriptBamu = 4
	ahScriptBeng = 5
	ahScriptBuhd = 6
	ahScriptCakm = 7
	ahScriptCans = 8
	ahScriptCari = 9
	ahScriptCher = 10
	ahScriptCopt = 11
	ahScriptCprt = 12
	ahScriptCyrl = 13
	ahScriptDeva = 14
	ahScriptDsrt = 15
	ahScriptEthi = 16
	ahScriptGeor = 17
	ahScriptGeok = 18
	ahScriptGlag = 19
	ahScriptGoth = 20
	ahScriptGrek = 21
	ahScriptGujr = 22
	ahScriptGuru = 23
	ahScriptHebr = 24
	ahScriptHmnp = 25
	ahScriptKali = 26
	ahScriptKhmr = 27
	ahScriptKhms = 28
	ahScriptKnda = 29
	ahScriptLaoo = 30
	ahScriptLatn = 31
	ahScriptLatb = 32
	ahScriptLatp = 33
	ahScriptLisu = 34
	ahScriptMlym = 35
	ahScriptMedf = 36
	ahScriptMong = 37
	ahScriptMymr = 38
	ahScriptNkoo = 39
	ahScriptNone = 40
	ahScriptOlck = 41
	ahScriptOrkh = 42
	ahScriptOsge = 43
	ahScriptOsma = 44
	ahScriptRohg = 45
	ahScriptSaur = 46
	ahScriptShaw = 47
	ahScriptSinh = 48
	ahScriptSund = 49
	ahScriptTaml = 50
	ahScriptTavt = 51
	ahScriptTelu = 52
	ahScriptTfng = 53
	ahScriptThai = 54
	ahScriptVaii = 55
	ahScriptLimb = 56
	ahScriptOrya = 57
	ahScriptSylo = 58
	ahScriptTibt = 59
	ahScriptHani = 60
)

// ahStyleClasses is STYLE_CLASSES of skrifa.
var ahStyleClasses = [...]ahStyleClass{
	{name: "Adlam", index: 0, script: &ahScriptClasses[0]},
	{name: "Arabic", index: 1, script: &ahScriptClasses[1]},
	{name: "Armenian", index: 2, script: &ahScriptClasses[2]},
	{name: "Avestan", index: 3, script: &ahScriptClasses[3]},
	{name: "Bamum", index: 4, script: &ahScriptClasses[4]},
	{name: "Bengali", index: 5, script: &ahScriptClasses[5]},
	{name: "Buhid", index: 6, script: &ahScriptClasses[6]},
	{name: "Chakma", index: 7, script: &ahScriptClasses[7]},
	{name: "Canadian Syllabics", index: 8, script: &ahScriptClasses[8]},
	{name: "Carian", index: 9, script: &ahScriptClasses[9]},
	{name: "Cherokee", index: 10, script: &ahScriptClasses[10]},
	{name: "Coptic", index: 11, script: &ahScriptClasses[11]},
	{name: "Cypriot", index: 12, script: &ahScriptClasses[12]},
	{name: "Cyrillic petite capitals from capitals", index: 13, script: &ahScriptClasses[13], feature: ahTag("c2cp"), hasFeature: true},
	{name: "Cyrillic small capitals from capitals", index: 14, script: &ahScriptClasses[13], feature: ahTag("c2sc"), hasFeature: true},
	{name: "Cyrillic ordinals", index: 15, script: &ahScriptClasses[13], feature: ahTag("ordn"), hasFeature: true},
	{name: "Cyrillic petite capitals", index: 16, script: &ahScriptClasses[13], feature: ahTag("pcap"), hasFeature: true},
	{name: "Cyrillic ruby", index: 17, script: &ahScriptClasses[13], feature: ahTag("ruby"), hasFeature: true},
	{name: "Cyrillic scientific inferiors", index: 18, script: &ahScriptClasses[13], feature: ahTag("sinf"), hasFeature: true},
	{name: "Cyrillic small capitals", index: 19, script: &ahScriptClasses[13], feature: ahTag("smcp"), hasFeature: true},
	{name: "Cyrillic subscript", index: 20, script: &ahScriptClasses[13], feature: ahTag("subs"), hasFeature: true},
	{name: "Cyrillic superscript", index: 21, script: &ahScriptClasses[13], feature: ahTag("sups"), hasFeature: true},
	{name: "Cyrillic titling", index: 22, script: &ahScriptClasses[13], feature: ahTag("titl"), hasFeature: true},
	{name: "Cyrillic", index: 23, script: &ahScriptClasses[13]},
	{name: "Devanagari", index: 24, script: &ahScriptClasses[14]},
	{name: "Deseret", index: 25, script: &ahScriptClasses[15]},
	{name: "Ethiopic", index: 26, script: &ahScriptClasses[16]},
	{name: "Georgian (Mkhedruli)", index: 27, script: &ahScriptClasses[17]},
	{name: "Georgian (Khutsuri)", index: 28, script: &ahScriptClasses[18]},
	{name: "Glagolitic", index: 29, script: &ahScriptClasses[19]},
	{name: "Gothic", index: 30, script: &ahScriptClasses[20]},
	{name: "Greek petite capitals from capitals", index: 31, script: &ahScriptClasses[21], feature: ahTag("c2cp"), hasFeature: true},
	{name: "Greek small capitals from capitals", index: 32, script: &ahScriptClasses[21], feature: ahTag("c2sc"), hasFeature: true},
	{name: "Greek ordinals", index: 33, script: &ahScriptClasses[21], feature: ahTag("ordn"), hasFeature: true},
	{name: "Greek petite capitals", index: 34, script: &ahScriptClasses[21], feature: ahTag("pcap"), hasFeature: true},
	{name: "Greek ruby", index: 35, script: &ahScriptClasses[21], feature: ahTag("ruby"), hasFeature: true},
	{name: "Greek scientific inferiors", index: 36, script: &ahScriptClasses[21], feature: ahTag("sinf"), hasFeature: true},
	{name: "Greek small capitals", index: 37, script: &ahScriptClasses[21], feature: ahTag("smcp"), hasFeature: true},
	{name: "Greek subscript", index: 38, script: &ahScriptClasses[21], feature: ahTag("subs"), hasFeature: true},
	{name: "Greek superscript", index: 39, script: &ahScriptClasses[21], feature: ahTag("sups"), hasFeature: true},
	{name: "Greek titling", index: 40, script: &ahScriptClasses[21], feature: ahTag("titl"), hasFeature: true},
	{name: "Greek", index: 41, script: &ahScriptClasses[21]},
	{name: "Gujarati", index: 42, script: &ahScriptClasses[22]},
	{name: "Gurmukhi", index: 43, script: &ahScriptClasses[23]},
	{name: "Hebrew", index: 44, script: &ahScriptClasses[24]},
	{name: "Nyiakeng Puachue Hmong", index: 45, script: &ahScriptClasses[25]},
	{name: "Kayah Li", index: 46, script: &ahScriptClasses[26]},
	{name: "Khmer", index: 47, script: &ahScriptClasses[27]},
	{name: "Khmer Symbols", index: 48, script: &ahScriptClasses[28]},
	{name: "Kannada", index: 49, script: &ahScriptClasses[29]},
	{name: "Lao", index: 50, script: &ahScriptClasses[30]},
	{name: "Latin petite capitals from capitals", index: 51, script: &ahScriptClasses[31], feature: ahTag("c2cp"), hasFeature: true},
	{name: "Latin small capitals from capitals", index: 52, script: &ahScriptClasses[31], feature: ahTag("c2sc"), hasFeature: true},
	{name: "Latin ordinals", index: 53, script: &ahScriptClasses[31], feature: ahTag("ordn"), hasFeature: true},
	{name: "Latin petite capitals", index: 54, script: &ahScriptClasses[31], feature: ahTag("pcap"), hasFeature: true},
	{name: "Latin ruby", index: 55, script: &ahScriptClasses[31], feature: ahTag("ruby"), hasFeature: true},
	{name: "Latin scientific inferiors", index: 56, script: &ahScriptClasses[31], feature: ahTag("sinf"), hasFeature: true},
	{name: "Latin small capitals", index: 57, script: &ahScriptClasses[31], feature: ahTag("smcp"), hasFeature: true},
	{name: "Latin subscript", index: 58, script: &ahScriptClasses[31], feature: ahTag("subs"), hasFeature: true},
	{name: "Latin superscript", index: 59, script: &ahScriptClasses[31], feature: ahTag("sups"), hasFeature: true},
	{name: "Latin titling", index: 60, script: &ahScriptClasses[31], feature: ahTag("titl"), hasFeature: true},
	{name: "Latin", index: 61, script: &ahScriptClasses[31]},
	{name: "Latin Subscript Fallback", index: 62, script: &ahScriptClasses[32]},
	{name: "Latin Superscript Fallback", index: 63, script: &ahScriptClasses[33]},
	{name: "Lisu", index: 64, script: &ahScriptClasses[34]},
	{name: "Malayalam", index: 65, script: &ahScriptClasses[35]},
	{name: "Medefaidrin", index: 66, script: &ahScriptClasses[36]},
	{name: "Mongolian", index: 67, script: &ahScriptClasses[37]},
	{name: "Myanmar", index: 68, script: &ahScriptClasses[38]},
	{name: "N'Ko", index: 69, script: &ahScriptClasses[39]},
	{name: "no script", index: 70, script: &ahScriptClasses[40]},
	{name: "Ol Chiki", index: 71, script: &ahScriptClasses[41]},
	{name: "Old Turkic", index: 72, script: &ahScriptClasses[42]},
	{name: "Osage", index: 73, script: &ahScriptClasses[43]},
	{name: "Osmanya", index: 74, script: &ahScriptClasses[44]},
	{name: "Hanifi Rohingya", index: 75, script: &ahScriptClasses[45]},
	{name: "Saurashtra", index: 76, script: &ahScriptClasses[46]},
	{name: "Shavian", index: 77, script: &ahScriptClasses[47]},
	{name: "Sinhala", index: 78, script: &ahScriptClasses[48]},
	{name: "Sundanese", index: 79, script: &ahScriptClasses[49]},
	{name: "Tamil", index: 80, script: &ahScriptClasses[50]},
	{name: "Tai Viet", index: 81, script: &ahScriptClasses[51]},
	{name: "Telugu", index: 82, script: &ahScriptClasses[52]},
	{name: "Tifinagh", index: 83, script: &ahScriptClasses[53]},
	{name: "Thai", index: 84, script: &ahScriptClasses[54]},
	{name: "Vai", index: 85, script: &ahScriptClasses[55]},
	{name: "Limbu", index: 86, script: &ahScriptClasses[56]},
	{name: "Oriya", index: 87, script: &ahScriptClasses[57]},
	{name: "Syloti Nagri", index: 88, script: &ahScriptClasses[58]},
	{name: "Tibetan", index: 89, script: &ahScriptClasses[59]},
	{name: "CJKV ideographs", index: 90, script: &ahScriptClasses[60]},
}

// Indices of the style classes.
const (
	ahStyleAdlm     = 0
	ahStyleArab     = 1
	ahStyleArmn     = 2
	ahStyleAvst     = 3
	ahStyleBamu     = 4
	ahStyleBeng     = 5
	ahStyleBuhd     = 6
	ahStyleCakm     = 7
	ahStyleCans     = 8
	ahStyleCari     = 9
	ahStyleCher     = 10
	ahStyleCopt     = 11
	ahStyleCprt     = 12
	ahStyleCyrlC2Cp = 13
	ahStyleCyrlC2Sc = 14
	ahStyleCyrlOrdn = 15
	ahStyleCyrlPcap = 16
	ahStyleCyrlRuby = 17
	ahStyleCyrlSinf = 18
	ahStyleCyrlSmcp = 19
	ahStyleCyrlSubs = 20
	ahStyleCyrlSups = 21
	ahStyleCyrlTitl = 22
	ahStyleCyrl     = 23
	ahStyleDeva     = 24
	ahStyleDsrt     = 25
	ahStyleEthi     = 26
	ahStyleGeor     = 27
	ahStyleGeok     = 28
	ahStyleGlag     = 29
	ahStyleGoth     = 30
	ahStyleGrekC2Cp = 31
	ahStyleGrekC2Sc = 32
	ahStyleGrekOrdn = 33
	ahStyleGrekPcap = 34
	ahStyleGrekRuby = 35
	ahStyleGrekSinf = 36
	ahStyleGrekSmcp = 37
	ahStyleGrekSubs = 38
	ahStyleGrekSups = 39
	ahStyleGrekTitl = 40
	ahStyleGrek     = 41
	ahStyleGujr     = 42
	ahStyleGuru     = 43
	ahStyleHebr     = 44
	ahStyleHmnp     = 45
	ahStyleKali     = 46
	ahStyleKhmr     = 47
	ahStyleKhms     = 48
	ahStyleKnda     = 49
	ahStyleLaoo     = 50
	ahStyleLatnC2Cp = 51
	ahStyleLatnC2Sc = 52
	ahStyleLatnOrdn = 53
	ahStyleLatnPcap = 54
	ahStyleLatnRuby = 55
	ahStyleLatnSinf = 56
	ahStyleLatnSmcp = 57
	ahStyleLatnSubs = 58
	ahStyleLatnSups = 59
	ahStyleLatnTitl = 60
	ahStyleLatn     = 61
	ahStyleLatb     = 62
	ahStyleLatp     = 63
	ahStyleLisu     = 64
	ahStyleMlym     = 65
	ahStyleMedf     = 66
	ahStyleMong     = 67
	ahStyleMymr     = 68
	ahStyleNkoo     = 69
	ahStyleNone     = 70
	ahStyleOlck     = 71
	ahStyleOrkh     = 72
	ahStyleOsge     = 73
	ahStyleOsma     = 74
	ahStyleRohg     = 75
	ahStyleSaur     = 76
	ahStyleShaw     = 77
	ahStyleSinh     = 78
	ahStyleSund     = 79
	ahStyleTaml     = 80
	ahStyleTavt     = 81
	ahStyleTelu     = 82
	ahStyleTfng     = 83
	ahStyleThai     = 84
	ahStyleVaii     = 85
	ahStyleLimb     = 86
	ahStyleOrya     = 87
	ahStyleSylo     = 88
	ahStyleTibt     = 89
	ahStyleHani     = 90
)

// ahStyleRanges is STYLE_RANGES of skrifa.
var ahStyleRanges = [...]ahStyleRange{
	{32, 93, 61},
	{94, 96, 61 | ahGlyphStyleNonBase},
	{97, 125, 61},
	{126, 126, 61 | ahGlyphStyleNonBase},
	{127, 127, 61},
	{160, 167, 61},
	{168, 169, 61 | ahGlyphStyleNonBase},
	{170, 170, 63},
	{171, 173, 61},
	{174, 176, 61 | ahGlyphStyleNonBase},
	{177, 177, 61},
	{178, 179, 63},
	{180, 180, 61 | ahGlyphStyleNonBase},
	{181, 183, 61},
	{184, 184, 61 | ahGlyphStyleNonBase},
	{185, 186, 63},
	{187, 187, 61},
	{188, 190, 61 | ahGlyphStyleNonBase},
	{191, 687, 61},
	{688, 696, 63},
	{697, 735, 61 | ahGlyphStyleNonBase},
	{736, 740, 63},
	{741, 879, 61 | ahGlyphStyleNonBase},
	{880, 889, 41},
	{890, 890, 41 | ahGlyphStyleNonBase},
	{891, 899, 41},
	{900, 901, 41 | ahGlyphStyleNonBase},
	{902, 1023, 41},
	{1024, 1154, 23},
	{1155, 1161, 23 | ahGlyphStyleNonBase},
	{1162, 1327, 23},
	{1328, 1368, 2},
	{1369, 1375, 2 | ahGlyphStyleNonBase},
	{1376, 1423, 2},
	{1424, 1424, 44},
	{1425, 1471, 44 | ahGlyphStyleNonBase},
	{1472, 1472, 44},
	{1473, 1474, 44 | ahGlyphStyleNonBase},
	{1475, 1475, 44},
	{1476, 1477, 44 | ahGlyphStyleNonBase},
	{1478, 1478, 44},
	{1479, 1479, 44 | ahGlyphStyleNonBase},
	{1480, 1535, 44},
	{1536, 1541, 1 | ahGlyphStyleNonBase},
	{1542, 1551, 1},
	{1552, 1562, 1 | ahGlyphStyleNonBase},
	{1563, 1610, 1},
	{1611, 1631, 1 | ahGlyphStyleNonBase},
	{1632, 1647, 1},
	{1648, 1648, 1 | ahGlyphStyleNonBase},
	{1649, 1749, 1},
	{1750, 1756, 1 | ahGlyphStyleNonBase},
	{1757, 1758, 1},
	{1759, 1764, 1 | ahGlyphStyleNonBase},
	{1765, 1766, 1},
	{1767, 1768, 1 | ahGlyphStyleNonBase},
	{1769, 1769, 1},
	{1770, 1773, 1 | ahGlyphStyleNonBase},
	{1774, 1791, 1},
	{1872, 2047, 1},
	{2208, 2258, 1},
	{2259, 2303, 1 | ahGlyphStyleNonBase},
	{2304, 2306, 24 | ahGlyphStyleNonBase},
	{2307, 2361, 24},
	{2362, 2362, 24 | ahGlyphStyleNonBase},
	{2363, 2363, 24},
	{2365, 2368, 24},
	{2369, 2376, 24 | ahGlyphStyleNonBase},
	{2377, 2380, 24},
	{2381, 2381, 24 | ahGlyphStyleNonBase},
	{2382, 2384, 24},
	{2387, 2391, 24 | ahGlyphStyleNonBase},
	{2392, 2401, 24},
	{2402, 2403, 24 | ahGlyphStyleNonBase},
	{2406, 2431, 24},
	{2432, 2432, 5},
	{2433, 2433, 5 | ahGlyphStyleNonBase},
	{2434, 2491, 5},
	{2492, 2492, 5 | ahGlyphStyleNonBase},
	{2493, 2496, 5},
	{2497, 2500, 5 | ahGlyphStyleNonBase},
	{2501, 2508, 5},
	{2509, 2509, 5 | ahGlyphStyleNonBase},
	{2510, 2529, 5},
	{2530, 2531, 5 | ahGlyphStyleNonBase},
	{2532, 2557, 5},
	{2558, 2558, 5 | ahGlyphStyleNonBase},
	{2559, 2559, 5},
	{2560, 2560, 43},
	{2561, 2562, 43 | ahGlyphStyleNonBase},
	{2563, 2619, 43},
	{2620, 2620, 43 | ahGlyphStyleNonBase},
	{2621, 2624, 43},
	{2625, 2641, 43 | ahGlyphStyleNonBase},
	{2642, 2671, 43},
	{2672, 2673, 43 | ahGlyphStyleNonBase},
	{2674, 2676, 43},
	{2677, 2677, 43 | ahGlyphStyleNonBase},
	{2678, 2687, 43},
	{2688, 2688, 42},
	{2689, 2690, 42 | ahGlyphStyleNonBase},
	{2691, 2747, 42},
	{2748, 2748, 42 | ahGlyphStyleNonBase},
	{2749, 2752, 42},
	{2753, 2760, 42 | ahGlyphStyleNonBase},
	{2761, 2764, 42},
	{2765, 2765, 42 | ahGlyphStyleNonBase},
	{2766, 2785, 42},
	{2786, 2787, 42 | ahGlyphStyleNonBase},
	{2788, 2809, 42},
	{2810, 2815, 42 | ahGlyphStyleNonBase},
	{2816, 2816, 87},
	{2817, 2818, 87 | ahGlyphStyleNonBase},
	{2819, 2875, 87},
	{2876, 2876, 87 | ahGlyphStyleNonBase},
	{2877, 2878, 87},
	{2879, 2879, 87 | ahGlyphStyleNonBase},
	{2880, 2880, 87},
	{2881, 2884, 87 | ahGlyphStyleNonBase},
	{2885, 2892, 87},
	{2893, 2902, 87 | ahGlyphStyleNonBase},
	{2903, 2913, 87},
	{2914, 2915, 87 | ahGlyphStyleNonBase},
	{2916, 2943, 87},
	{2944, 2945, 80},
	{2946, 2946, 80 | ahGlyphStyleNonBase},
	{2947, 3007, 80},
	{3008, 3010, 80 | ahGlyphStyleNonBase},
	{3011, 3020, 80},
	{3021, 3021, 80 | ahGlyphStyleNonBase},
	{3022, 3071, 80},
	{3072, 3072, 82 | ahGlyphStyleNonBase},
	{3073, 3075, 82},
	{3076, 3076, 82 | ahGlyphStyleNonBase},
	{3077, 3133, 82},
	{3134, 3136, 82 | ahGlyphStyleNonBase},
	{3137, 3141, 82},
	{3142, 3158, 82 | ahGlyphStyleNonBase},
	{3159, 3169, 82},
	{3170, 3171, 82 | ahGlyphStyleNonBase},
	{3172, 3199, 82},
	{3200, 3200, 49},
	{3201, 3201, 49 | ahGlyphStyleNonBase},
	{3202, 3259, 49},
	{3260, 3260, 49 | ahGlyphStyleNonBase},
	{3261, 3262, 49},
	{3263, 3263, 49 | ahGlyphStyleNonBase},
	{3264, 3269, 49},
	{3270, 3270, 49 | ahGlyphStyleNonBase},
	{3271, 3275, 49},
	{3276, 3277, 49 | ahGlyphStyleNonBase},
	{3278, 3297, 49},
	{3298, 3299, 49 | ahGlyphStyleNonBase},
	{3300, 3327, 49},
	{3328, 3329, 65 | ahGlyphStyleNonBase},
	{3330, 3386, 65},
	{3387, 3388, 65 | ahGlyphStyleNonBase},
	{3389, 3404, 65},
	{3405, 3406, 65 | ahGlyphStyleNonBase},
	{3407, 3425, 65},
	{3426, 3427, 65 | ahGlyphStyleNonBase},
	{3428, 3455, 65},
	{3456, 3529, 78},
	{3530, 3530, 78 | ahGlyphStyleNonBase},
	{3531, 3537, 78},
	{3538, 3542, 78 | ahGlyphStyleNonBase},
	{3543, 3583, 78},
	{3584, 3632, 84},
	{3633, 3633, 84 | ahGlyphStyleNonBase},
	{3634, 3635, 84},
	{3636, 3642, 84 | ahGlyphStyleNonBase},
	{3643, 3654, 84},
	{3655, 3662, 84 | ahGlyphStyleNonBase},
	{3663, 3711, 84},
	{3712, 3760, 50},
	{3761, 3761, 50 | ahGlyphStyleNonBase},
	{3762, 3763, 50},
	{3764, 3772, 50 | ahGlyphStyleNonBase},
	{3773, 3783, 50},
	{3784, 3789, 50 | ahGlyphStyleNonBase},
	{3790, 3839, 50},
	{3840, 3863, 89},
	{3864, 3865, 89 | ahGlyphStyleNonBase},
	{3866, 3892, 89},
	{3893, 3893, 89 | ahGlyphStyleNonBase},
	{3894, 3894, 89},
	{3895, 3895, 89 | ahGlyphStyleNonBase},
	{3896, 3896, 89},
	{3897, 3897, 89 | ahGlyphStyleNonBase},
	{3898, 3901, 89},
	{3902, 3903, 89 | ahGlyphStyleNonBase},
	{3904, 3952, 89},
	{3953, 3966, 89 | ahGlyphStyleNonBase},
	{3967, 3967, 89},
	{3968, 3972, 89 | ahGlyphStyleNonBase},
	{3973, 3973, 89},
	{3974, 3975, 89 | ahGlyphStyleNonBase},
	{3976, 3980, 89},
	{3981, 4028, 89 | ahGlyphStyleNonBase},
	{4029, 4095, 89},
	{4096, 4140, 68},
	{4141, 4144, 68 | ahGlyphStyleNonBase},
	{4145, 4145, 68},
	{4146, 4151, 68 | ahGlyphStyleNonBase},
	{4152, 4153, 68},
	{4154, 4154, 68 | ahGlyphStyleNonBase},
	{4155, 4156, 68},
	{4157, 4158, 68 | ahGlyphStyleNonBase},
	{4159, 4183, 68},
	{4184, 4185, 68 | ahGlyphStyleNonBase},
	{4186, 4189, 68},
	{4190, 4192, 68 | ahGlyphStyleNonBase},
	{4193, 4208, 68},
	{4209, 4212, 68 | ahGlyphStyleNonBase},
	{4213, 4225, 68},
	{4226, 4226, 68 | ahGlyphStyleNonBase},
	{4227, 4228, 68},
	{4229, 4230, 68 | ahGlyphStyleNonBase},
	{4231, 4236, 68},
	{4237, 4237, 68 | ahGlyphStyleNonBase},
	{4238, 4255, 68},
	{4256, 4301, 28},
	{4304, 4351, 27},
	{4352, 4607, 90},
	{4608, 4956, 26},
	{4957, 4959, 26 | ahGlyphStyleNonBase},
	{4960, 5023, 26},
	{5024, 5119, 10},
	{5120, 5759, 8},
	{5952, 5969, 6},
	{5970, 5971, 6 | ahGlyphStyleNonBase},
	{5972, 5983, 6},
	{6016, 6070, 47},
	{6071, 6077, 47 | ahGlyphStyleNonBase},
	{6078, 6085, 47},
	{6086, 6086, 47 | ahGlyphStyleNonBase},
	{6087, 6088, 47},
	{6089, 6099, 47 | ahGlyphStyleNonBase},
	{6100, 6108, 47},
	{6109, 6109, 47 | ahGlyphStyleNonBase},
	{6110, 6143, 47},
	{6144, 6276, 67},
	{6277, 6278, 67 | ahGlyphStyleNonBase},
	{6279, 6312, 67},
	{6313, 6313, 67 | ahGlyphStyleNonBase},
	{6314, 6319, 67},
	{6320, 6399, 8},
	{6400, 6431, 86},
	{6432, 6434, 86 | ahGlyphStyleNonBase},
	{6435, 6438, 86},
	{6439, 6452, 86 | ahGlyphStyleNonBase},
	{6453, 6454, 86},
	{6455, 6459, 86 | ahGlyphStyleNonBase},
	{6460, 6479, 86},
	{6624, 6655, 48},
	{6832, 6846, 61 | ahGlyphStyleNonBase},
	{7040, 7042, 79 | ahGlyphStyleNonBase},
	{7043, 7072, 79},
	{7073, 7085, 79 | ahGlyphStyleNonBase},
	{7086, 7103, 79},
	{7248, 7295, 71},
	{7296, 7311, 23},
	{7312, 7359, 27},
	{7360, 7375, 79},
	{7424, 7467, 61},
	{7468, 7521, 63},
	{7522, 7530, 62},
	{7531, 7543, 61},
	{7544, 7544, 63},
	{7545, 7578, 61},
	{7579, 7615, 63},
	{7616, 7679, 61 | ahGlyphStyleNonBase},
	{7680, 7935, 61},
	{7936, 8124, 41},
	{8125, 8129, 41 | ahGlyphStyleNonBase},
	{8130, 8140, 41},
	{8141, 8143, 41 | ahGlyphStyleNonBase},
	{8144, 8156, 41},
	{8157, 8159, 41 | ahGlyphStyleNonBase},
	{8160, 8172, 41},
	{8173, 8175, 41 | ahGlyphStyleNonBase},
	{8176, 8188, 41},
	{8189, 8190, 41 | ahGlyphStyleNonBase},
	{8191, 8191, 41},
	{8192, 8214, 61},
	{8215, 8215, 61 | ahGlyphStyleNonBase},
	{8216, 8253, 61},
	{8254, 8254, 61 | ahGlyphStyleNonBase},
	{8255, 8303, 61},
	{8304, 8319, 63},
	{8320, 8348, 62},
	{8352, 8376, 61},
	{8377, 8377, 24},
	{8378, 8399, 61},
	{8528, 8591, 61},
	{11264, 11359, 29},
	{11360, 11387, 61},
	{11388, 11388, 62},
	{11389, 11389, 63},
	{11390, 11391, 61},
	{11392, 11502, 11},
	{11503, 11505, 11 | ahGlyphStyleNonBase},
	{11506, 11519, 11},
	{11520, 11565, 28},
	{11568, 11647, 83},
	{11648, 11743, 26},
	{11744, 11775, 23 | ahGlyphStyleNonBase},
	{11776, 11903, 61},
	{11904, 12255, 90},
	{12272, 12329, 90},
	{12330, 12335, 90 | ahGlyphStyleNonBase},
	{12336, 12687, 90},
	{12688, 12703, 90 | ahGlyphStyleNonBase},
	{12704, 12799, 90},
	{13056, 40959, 90},
	{42192, 42239, 64},
	{42240, 42559, 85},
	{42560, 42606, 23},
	{42607, 42623, 23 | ahGlyphStyleNonBase},
	{42624, 42653, 23},
	{42654, 42655, 23 | ahGlyphStyleNonBase},
	{42656, 42735, 4},
	{42736, 42737, 4 | ahGlyphStyleNonBase},
	{42738, 42751, 4},
	{42784, 42863, 61},
	{42864, 42864, 63},
	{42865, 42887, 61},
	{42888, 42888, 61 | ahGlyphStyleNonBase},
	{42889, 42999, 61},
	{43000, 43001, 63},
	{43002, 43002, 61 | ahGlyphStyleNonBase},
	{43003, 43007, 61},
	{43008, 43009, 88},
	{43010, 43010, 88 | ahGlyphStyleNonBase},
	{43011, 43013, 88},
	{43014, 43014, 88 | ahGlyphStyleNonBase},
	{43015, 43018, 88},
	{43019, 43019, 88 | ahGlyphStyleNonBase},
	{43020, 43044, 88},
	{43045, 43046, 88 | ahGlyphStyleNonBase},
	{43047, 43055, 88},
	{43136, 43137, 76 | ahGlyphStyleNonBase},
	{43138, 43187, 76},
	{43188, 43205, 76 | ahGlyphStyleNonBase},
	{43206, 43231, 76},
	{43232, 43249, 24 | ahGlyphStyleNonBase},
	{43250, 43262, 24},
	{43263, 43263, 24 | ahGlyphStyleNonBase},
	{43264, 43301, 46},
	{43302, 43309, 46 | ahGlyphStyleNonBase},
	{43310, 43311, 46},
	{43360, 43391, 90},
	{43488, 43492, 68},
	{43493, 43493, 68 | ahGlyphStyleNonBase},
	{43494, 43519, 68},
	{43616, 43643, 68},
	{43644, 43644, 68 | ahGlyphStyleNonBase},
	{43645, 43647, 68},
	{43648, 43695, 81},
	{43696, 43696, 81 | ahGlyphStyleNonBase},
	{43697, 43697, 81},
	{43698, 43700, 81 | ahGlyphStyleNonBase},
	{43701, 43702, 81},
	{43703, 43704, 81 | ahGlyphStyleNonBase},
	{43705, 43709, 81},
	{43710, 43711, 81 | ahGlyphStyleNonBase},
	{43712, 43712, 81},
	{43713, 43713, 81 | ahGlyphStyleNonBase},
	{43714, 43743, 81},
	{43776, 43823, 26},
	{43824, 43867, 61},
	{43868, 43871, 63},
	{43872, 43887, 61},
	{43888, 43967, 10},
	{44032, 55295, 90},
	{63744, 64255, 90},
	{64256, 64262, 61},
	{64275, 64279, 2},
	{64285, 64285, 44},
	{64286, 64286, 44 | ahGlyphStyleNonBase},
	{64287, 64335, 44},
	{64336, 64433, 1},
	{64434, 64449, 1 | ahGlyphStyleNonBase},
	{64450, 65023, 1},
	{65040, 65055, 90},
	{65072, 65103, 90},
	{65136, 65136, 1 | ahGlyphStyleNonBase},
	{65137, 65137, 1},
	{65138, 65138, 1 | ahGlyphStyleNonBase},
	{65139, 65139, 1},
	{65140, 65140, 1 | ahGlyphStyleNonBase},
	{65141, 65141, 1},
	{65142, 65142, 1 | ahGlyphStyleNonBase},
	{65143, 65143, 1},
	{65144, 65144, 1 | ahGlyphStyleNonBase},
	{65145, 65145, 1},
	{65146, 65146, 1 | ahGlyphStyleNonBase},
	{65147, 65147, 1},
	{65148, 65148, 1 | ahGlyphStyleNonBase},
	{65149, 65149, 1},
	{65150, 65150, 1 | ahGlyphStyleNonBase},
	{65151, 65279, 1},
	{65280, 65519, 90},
	{66208, 66271, 9},
	{66352, 66383, 30},
	{66560, 66639, 25},
	{66640, 66687, 77},
	{66688, 66735, 74},
	{66736, 66815, 73},
	{67584, 67647, 12},
	{68352, 68408, 3},
	{68409, 68415, 3 | ahGlyphStyleNonBase},
	{68608, 68687, 72},
	{68864, 68927, 75},
	{69888, 69890, 7 | ahGlyphStyleNonBase},
	{69891, 69926, 7},
	{69927, 69940, 7 | ahGlyphStyleNonBase},
	{69941, 69957, 7},
	{69958, 69958, 7 | ahGlyphStyleNonBase},
	{69959, 69967, 7},
	{71264, 71295, 67},
	{93760, 93855, 66},
	{110592, 110895, 90},
	{119552, 119647, 90},
	{119808, 120831, 61},
	{122880, 122927, 29 | ahGlyphStyleNonBase},
	{123136, 123215, 45},
	{125184, 125258, 0 | ahGlyphStyleNonBase},
	{125259, 125279, 0},
	{126464, 126719, 1},
	{131072, 173791, 90},
	{173824, 191471, 90},
	{194560, 195103, 90},
}
