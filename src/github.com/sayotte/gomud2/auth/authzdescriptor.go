package auth

type AuthZDescriptor struct {
	CreateCharacter  bool
	PossessCharacter bool
	MayLogin         bool
	ServerOperations bool
}
