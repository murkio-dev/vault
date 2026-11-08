package utils

import (
	"fmt"
	"math/rand"
)

const (
	alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"
	middleChars  = "abcdefghijklmnopqrstuvwxyz0123456789-"
)

func GenRandomApplicationName() string {
	// 3 a 50 caracteres no total (o limite do nome de uma aplicação).
	middleLen := rand.Intn(47) + 1
	result := make([]byte, 1+middleLen+1)
	result[0] = alphanumeric[rand.Intn(len(alphanumeric))]

	for i := 1; i <= middleLen; i++ {
		result[i] = middleChars[rand.Intn(len(middleChars))]
	}

	result[1+middleLen] = alphanumeric[rand.Intn(len(alphanumeric))]

	return string(result)
}

func GenRandomIP(ipType string) string {
	var o1 int

	o2 := rand.Intn(256)
	o3 := rand.Intn(256)
	o4 := rand.Intn(256)

	switch ipType {
	case "a":
		o1 = rand.Intn(126) + 1
	case "b":
		o1 = rand.Intn(64) + 128
	case "c":
		o1 = rand.Intn(32) + 192
	case "d":
		o1 = rand.Intn(16) + 224
	case "e":
		o1 = rand.Intn(15) + 240
	default:
		o1 = rand.Intn(32) + 192
	}

	return fmt.Sprintf("%d.%d.%d.%d", o1, o2, o3, o4)
}

func GenRandomLetterAtoE() string {
	return string(byte('a' + rand.Intn(5)))
}

func GenRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:124.0) Gecko/20100101 Firefox/124.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36 Edg/123.0.0.0",
	}

	return userAgents[rand.Intn(len(userAgents))]
}

func GenRandomCountry() string {
	countries := []string{
		"Brazil",
		"United States",
		"Canada",
		"Germany",
		"Japan",
		"United Kingdom",
		"France",
		"Australia",
		"India",
		"Argentina",
		"Spain",
		"Italy",
	}

	return countries[rand.Intn(len(countries))]
}

func GenRandomOrigin() string {
	origins := []string{
		"https://application.murkio.dev",
		"https://dashboard.murkio.dev",
		"https://staging.murkio.dev",
		"http://localhost:3000",
		"http://localhost:8080",
	}

	return origins[rand.Intn(len(origins))]
}

func GenRandomCdnRayID() string {
	const hexChars = "0123456789abcdef"
	bytes := make([]byte, 16)
	for i := range bytes {
		bytes[i] = hexChars[rand.Intn(len(hexChars))]
	}

	pops := []string{"GRU", "GIG", "EZE", "MIA", "LHR"}
	pop := pops[rand.Intn(len(pops))]

	return fmt.Sprintf("%s-%s", string(bytes), pop)
}

const upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenRandomSecretName() string {
	size := rand.Intn(20) + 2
	name := make([]byte, size)
	name[0] = upperChars[rand.Intn(len(upperChars))]
	for i := 1; i < size; i++ {
		name[i] = (upperChars + "0123456789_")[rand.Intn(len(upperChars)+11)]
	}
	return string(name)
}

func GenRandomSecretValue() string {
	size := rand.Intn(40) + 8
	value := make([]byte, size)
	for i := range value {
		value[i] = alphanumeric[rand.Intn(len(alphanumeric))]
	}
	return "v-" + string(value)
}

func GenRandomEmail() string {
	return GenRandomSecretValue()[:10] + "@example.com"
}

func GenRandomTokenName() string {
	return "token-" + GenRandomSecretValue()[:8]
}
