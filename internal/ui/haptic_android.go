//go:build android

package ui

/*
#include <jni.h>
#include <stdint.h>

// tap gives the tap feedback of the activity's window: the system's
// own, as the phone's touch feedback setting has it.
static void tap(uintptr_t envp, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv *)envp;
	jobject activity = (jobject)ctx;
	jclass cls = (*env)->GetObjectClass(env, activity);
	jmethodID getWindow = (*env)->GetMethodID(env, cls, "getWindow", "()Landroid/view/Window;");
	if (getWindow == NULL) {
		(*env)->ExceptionClear(env);
		return;
	}
	jobject window = (*env)->CallObjectMethod(env, activity, getWindow);
	if (window == NULL) {
		(*env)->ExceptionClear(env);
		return;
	}
	jclass wcls = (*env)->GetObjectClass(env, window);
	jmethodID getDecorView = (*env)->GetMethodID(env, wcls, "getDecorView", "()Landroid/view/View;");
	jobject view = getDecorView ? (*env)->CallObjectMethod(env, window, getDecorView) : NULL;
	if (view != NULL) {
		jclass vcls = (*env)->GetObjectClass(env, view);
		jmethodID perform = (*env)->GetMethodID(env, vcls, "performHapticFeedback", "(I)Z");
		if (perform != NULL) {
			// HapticFeedbackConstants.VIRTUAL_KEY
			(*env)->CallBooleanMethod(env, view, perform, 1);
		}
	}
	(*env)->ExceptionClear(env);
}
*/
import "C"

import "fyne.io/fyne/v2/driver"

// tapFeedback gives the phone's tap feedback (a short vibration, as its
// touch feedback setting has it), without holding up the caller.
func tapFeedback() {
	go func() {
		_ = driver.RunNative(func(ctx any) error {
			if c, ok := ctx.(*driver.AndroidContext); ok {
				C.tap(C.uintptr_t(c.Env), C.uintptr_t(c.Ctx))
			}
			return nil
		})
	}()
}
