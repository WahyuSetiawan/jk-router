// Default media provider registrations.
package media

func init() {
	// OpenAI: TTS + STT + Image
	Register(&Registry{
		ID:           "openai",
		Name:         "OpenAI",
		Capabilities: CapTTS | CapSTT | CapImage,
		// TTS: https://platform.openai.com/docs/api-reference/audio/createSpeech
		TTSBaseURL:    "https://api.openai.com",
		TTSPath:       "/v1/audio/speech",
		TTSAuthHeader: "Authorization",
		TTSAuthPrefix: "Bearer",
		// STT: https://platform.openai.com/docs/api-reference/audio/createTranscription
		STTBaseURL:    "https://api.openai.com",
		STTPath:       "/v1/audio/transcriptions",
		STTAuthHeader: "Authorization",
		STTAuthPrefix: "Bearer",
		// Image: https://platform.openai.com/docs/api-reference/images/generate
		ImageBaseURL:    "https://api.openai.com",
		ImagePath:       "/v1/images/generations",
		ImageAuthHeader: "Authorization",
		ImageAuthPrefix: "Bearer",
	})

	// ElevenLabs: TTS only
	Register(&Registry{
		ID:           "elevenlabs",
		Name:         "ElevenLabs",
		Capabilities: CapTTS,
		TTSBaseURL:    "https://api.elevenlabs.io",
		TTSPath:       "/v1/text-to-speech/{voice_id}",
		TTSAuthHeader: "xi-api-key",
		TTSAuthPrefix: "",
	})

	// Stability AI: Image only
	Register(&Registry{
		ID:           "stability",
		Name:         "Stability AI",
		Capabilities: CapImage,
		ImageBaseURL:    "https://api.stability.ai",
		ImagePath:       "/v1/generation/stable-diffusion-xl-1024-v1-0/text-to-image",
		ImageAuthHeader: "Authorization",
		ImageAuthPrefix: "Bearer",
	})
}
