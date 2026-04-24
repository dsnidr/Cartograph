package assets

import (
	"fmt"
)

// ResolutionPhase represents a specific stage in the asset resolution pipeline.
type ResolutionPhase string

const (
	// PhaseLoadBlockState occurs while reading the blockstate JSON.
	PhaseLoadBlockState ResolutionPhase = "LoadBlockState"
	// PhaseLoadModel occurs while reading a model JSON.
	PhaseLoadModel ResolutionPhase = "LoadModel"
	// PhaseFindModelVariant occurs while choosing a variant from a blockstate.
	PhaseFindModelVariant ResolutionPhase = "FindModelVariant"
	// PhaseExtractTextures occurs while extracting texture references from a model.
	PhaseExtractTextures ResolutionPhase = "ExtractTextures"
	// PhaseResolveVariables occurs while resolving `#texture` variables.
	PhaseResolveVariables ResolutionPhase = "ResolveVariables"
	// PhaseSelectCandidate occurs while selecting the best top-down texture candidate.
	PhaseSelectCandidate ResolutionPhase = "SelectCandidate"
	// PhaseReadTexture occurs while opening the final PNG file.
	PhaseReadTexture ResolutionPhase = "ReadTexture"
	// PhaseDecodeTexture occurs while decoding the PNG data.
	PhaseDecodeTexture ResolutionPhase = "DecodeTexture"
)

// Error provides rich context about failures during asset resolution.
type Error struct {
	BlockID  string
	Resource string
	Phase    ResolutionPhase
	Expected string
	Err      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	msg := fmt.Sprintf("[%s] failed resolving block '%s' at resource '%s'", e.Phase, e.BlockID, e.Resource)
	if e.Expected != "" {
		msg += fmt.Sprintf(" (expected: %s)", e.Expected)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}

	return msg
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the target error matches the criteria of this Error.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}

	// Match if the target has specific fields set
	if t.Phase != "" && t.Phase != e.Phase {
		return false
	}

	if t.BlockID != "" && t.BlockID != e.BlockID {
		return false
	}

	return true
}

func wrapError(err error, blockID, resource string, phase ResolutionPhase, expected string) error {
	if err == nil {
		return nil
	}

	return &Error{
		BlockID:  blockID,
		Resource: resource,
		Phase:    phase,
		Expected: expected,
		Err:      err,
	}
}

func newError(blockID, resource string, phase ResolutionPhase, expected string) error {
	return &Error{
		BlockID:  blockID,
		Resource: resource,
		Phase:    phase,
		Expected: expected,
	}
}
