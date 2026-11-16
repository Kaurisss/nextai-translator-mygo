package desktop

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var loadVision = sync.OnceValue(func() error {
	_, err := purego.Dlopen("/System/Library/Frameworks/Vision.framework/Vision", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	return err
})

func OCR(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	if err := loadMac(); err != nil {
		return "", err
	}
	if err := loadVision(); err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	request := objc.ID(objc.GetClass("VNRecognizeTextRequest")).Send(objc.RegisterName("new"))
	defer request.Send(objc.RegisterName("release"))
	request.Send(objc.RegisterName("setRecognitionLevel:"), 0) // VNRequestTextRecognitionLevelAccurate
	request.Send(objc.RegisterName("setUsesLanguageCorrection:"), true)
	var nativeError objc.ID
	supported := request.Send(objc.RegisterName("supportedRecognitionLanguagesAndReturnError:"), &nativeError)
	if supported == 0 {
		return "", visionError(nativeError)
	}
	languages := objc.ID(objc.GetClass("NSMutableArray")).Send(objc.RegisterName("array"))
	for _, language := range []string{"zh-Hans", "zh-Hant", "en-US"} {
		value := macString(language)
		if objc.Send[bool](supported, objc.RegisterName("containsObject:"), value) {
			languages.Send(objc.RegisterName("addObject:"), value)
		}
	}
	if languages.Send(objc.RegisterName("count")) > 0 {
		request.Send(objc.RegisterName("setRecognitionLanguages:"), languages)
	}
	url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), macString(path))
	options := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary"))
	handler := objc.ID(objc.GetClass("VNImageRequestHandler")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithURL:options:"), url, options)
	defer handler.Send(objc.RegisterName("release"))
	requests := objc.ID(objc.GetClass("NSArray")).Send(objc.RegisterName("arrayWithObject:"), request)
	if !objc.Send[bool](handler, objc.RegisterName("performRequests:error:"), requests, &nativeError) {
		return "", visionError(nativeError)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	results := request.Send(objc.RegisterName("results"))
	var lines []string
	for i, count := uintptr(0), uintptr(results.Send(objc.RegisterName("count"))); i < count; i++ {
		observation := results.Send(objc.RegisterName("objectAtIndex:"), i)
		candidate := observation.Send(objc.RegisterName("topCandidates:"), 1).Send(objc.RegisterName("firstObject"))
		if candidate != 0 {
			line := candidate.Send(objc.RegisterName("string"))
			lines = append(lines, objc.Send[string](line, objc.RegisterName("UTF8String")))
		}
	}
	return joinCJKSpaces(strings.Join(lines, "\n")), nil
}

func visionError(err objc.ID) error {
	if err == 0 {
		return errors.New("系统 Vision 文字识别失败")
	}
	return errors.New(objc.Send[string](err.Send(objc.RegisterName("localizedDescription")), objc.RegisterName("UTF8String")))
}
