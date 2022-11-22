package crypto

import "github.com/dobyte/due/log"

type Verifier interface {
	// Name 名称
	Name() string
	// Verify 验签
	Verify(data []byte, signature []byte) (bool, error)
}

var verifiers = make(map[string]Verifier)

// RegisterVerifier 注册验签器
func RegisterVerifier(verifier Verifier) {
	if verifier == nil {
		log.Fatal("can't register a invalid verifier")
	}

	name := verifier.Name()

	if name == "" {
		log.Fatal("can't register a verifier without name")
	}

	if _, ok := verifiers[name]; ok {
		log.Warnf("the old %s verifier will be overwritten", name)
	}

	verifiers[name] = verifier
}

// InvokeVerifier 调用验签器
func InvokeVerifier(name string) Verifier {
	verifier, ok := verifiers[name]
	if !ok {
		log.Fatalf("%s verifier is not registered", name)
	}

	return verifier
}
