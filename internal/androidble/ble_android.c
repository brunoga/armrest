//go:build android

// JNI glue between Go and Android's Bluetooth: see ble_android.go.

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>
#include "_cgo_export.h"

#define LOG(...) __android_log_print(ANDROID_LOG_WARN, "ArmrestBLE", __VA_ARGS__)

static JavaVM *vm;
static jobject ctx;          // the activity, a global reference
static jclass bridge;        // BleBridge, a global reference
static int sdk;              // Build.VERSION.SDK_INT
static int target;           // the app's targetSdkVersion

// env attaches the calling thread to the VM if it is not; *attached says
// whether to detach it again.
static JNIEnv *env(int *attached) {
	JNIEnv *e = NULL;
	*attached = 0;
	if ((*vm)->GetEnv(vm, (void **)&e, JNI_VERSION_1_6) == JNI_EDETACHED) {
		if ((*vm)->AttachCurrentThread(vm, &e, NULL) != 0) {
			return NULL;
		}
		*attached = 1;
	}
	return e;
}

static void release(int attached) {
	if (attached) {
		(*vm)->DetachCurrentThread(vm);
	}
}

// failed reports, and clears, a pending Java exception.
static int failed(JNIEnv *e) {
	if ((*e)->ExceptionCheck(e)) {
		LOG("Java exception:");
		(*e)->ExceptionDescribe(e);
		(*e)->ExceptionClear(e);
		return 1;
	}
	return 0;
}

static void nOnState(JNIEnv *e, jclass c, jlong h, jint status, jint state) { goBleState(h, status, state); }
static void nOnServices(JNIEnv *e, jclass c, jlong h, jint status) { goBleServices(h, status); }
static void nOnMtu(JNIEnv *e, jclass c, jlong h, jint mtu, jint status) { goBleMtu(h, mtu, status); }
static void nOnWrite(JNIEnv *e, jclass c, jlong h, jint status) { goBleWrite(h, status); }
static void nOnDescriptor(JNIEnv *e, jclass c, jlong h, jint status) { goBleDescriptor(h, status); }

static void nOnNotify(JNIEnv *e, jclass c, jlong h, jbyteArray value) {
	if (value == NULL) {
		return;
	}
	jsize n = (*e)->GetArrayLength(e, value);
	jbyte *b = (*e)->GetByteArrayElements(e, value, NULL);
	goBleNotify(h, b, n);
	(*e)->ReleaseByteArrayElements(e, value, b, JNI_ABORT);
}

static JNINativeMethod natives[] = {
	{"onState", "(JII)V", (void *)nOnState},
	{"onServices", "(JI)V", (void *)nOnServices},
	{"onMtu", "(JII)V", (void *)nOnMtu},
	{"onWrite", "(JI)V", (void *)nOnWrite},
	{"onDescriptor", "(JI)V", (void *)nOnDescriptor},
	{"onNotify", "(J[B)V", (void *)nOnNotify},
};

// ble_init loads BleBridge from its dex and registers its natives:
// in memory from Android 8, else from a read-only file in the app's code
// cache. It returns an error message, or NULL.
const char *ble_init(uintptr_t javaVM, uintptr_t activity, void *dex, int dexLen) {
	vm = (JavaVM *)javaVM;
	int attached;
	JNIEnv *e = env(&attached);
	if (e == NULL) {
		return "attaching to the Java VM";
	}
	const char *err = NULL;
	ctx = (*e)->NewGlobalRef(e, (jobject)activity);

	jclass version = (*e)->FindClass(e, "android/os/Build$VERSION");
	sdk = (*e)->GetStaticIntField(e, version, (*e)->GetStaticFieldID(e, version, "SDK_INT", "I"));

	jclass ctxClass = (*e)->GetObjectClass(e, ctx);
	jobject info = (*e)->CallObjectMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "getApplicationInfo", "()Landroid/content/pm/ApplicationInfo;"));
	if (!failed(e) && info != NULL) {
		jclass ic = (*e)->GetObjectClass(e, info);
		target = (*e)->GetIntField(e, info, (*e)->GetFieldID(e, ic, "targetSdkVersion", "I"));
	}
	jobject parent = (*e)->CallObjectMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "getClassLoader", "()Ljava/lang/ClassLoader;"));
	jobject loader = NULL;
	if (sdk >= 26) {
		jobject buf = (*e)->NewDirectByteBuffer(e, dex, dexLen);
		jclass imcl = (*e)->FindClass(e, "dalvik/system/InMemoryDexClassLoader");
		loader = (*e)->NewObject(e, imcl, (*e)->GetMethodID(e, imcl, "<init>", "(Ljava/nio/ByteBuffer;Ljava/lang/ClassLoader;)V"), buf, parent);
	} else {
		jobject dir = (*e)->CallObjectMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "getCodeCacheDir", "()Ljava/io/File;"));
		jclass fileClass = (*e)->FindClass(e, "java/io/File");
		jstring name = (*e)->NewStringUTF(e, "armrest-ble.dex");
		jobject file = (*e)->NewObject(e, fileClass, (*e)->GetMethodID(e, fileClass, "<init>", "(Ljava/io/File;Ljava/lang/String;)V"), dir, name);
		jstring path = (*e)->CallObjectMethod(e, file, (*e)->GetMethodID(e, fileClass, "getAbsolutePath", "()Ljava/lang/String;"));
		if (failed(e)) {
			err = "finding the code cache";
			goto out;
		}
		jclass fos = (*e)->FindClass(e, "java/io/FileOutputStream");
		jobject out = (*e)->NewObject(e, fos, (*e)->GetMethodID(e, fos, "<init>", "(Ljava/lang/String;)V"), path);
		jbyteArray bytes = (*e)->NewByteArray(e, dexLen);
		(*e)->SetByteArrayRegion(e, bytes, 0, dexLen, dex);
		(*e)->CallVoidMethod(e, out, (*e)->GetMethodID(e, fos, "write", "([B)V"), bytes);
		(*e)->CallVoidMethod(e, out, (*e)->GetMethodID(e, fos, "close", "()V"));
		(*e)->CallBooleanMethod(e, file, (*e)->GetMethodID(e, fileClass, "setReadOnly", "()Z"));
		jclass dcl = (*e)->FindClass(e, "dalvik/system/DexClassLoader");
		loader = (*e)->NewObject(e, dcl, (*e)->GetMethodID(e, dcl, "<init>", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/ClassLoader;)V"), path, NULL, NULL, parent);
	}
	if (failed(e) || loader == NULL) {
		err = "loading the Bluetooth bridge";
		goto out;
	}
	jclass loaderClass = (*e)->GetObjectClass(e, loader);
	jstring className = (*e)->NewStringUTF(e, "io.github.brunoga.armrest.BleBridge");
	jclass c = (*e)->CallObjectMethod(e, loader, (*e)->GetMethodID(e, loaderClass, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;"), className);
	if (failed(e) || c == NULL) {
		err = "loading BleBridge";
		goto out;
	}
	if ((*e)->RegisterNatives(e, c, natives, sizeof(natives) / sizeof(natives[0])) != 0 || failed(e)) {
		err = "registering BleBridge's natives";
		goto out;
	}
	bridge = (*e)->NewGlobalRef(e, c);
	LOG("init: bridge loaded (sdk %d, target %d)", sdk, target);
out:
	if (err != NULL) {
		LOG("init: %s", err);
	}
	release(attached);
	return err;
}

// ble_permission is 1 if the app may connect to Bluetooth devices. From
// Android 12 that is "Nearby devices" (BLUETOOTH_CONNECT), a run-time
// permission Android enforces whatever API level the app targets (our
// phone, Android API 37, did for ours, which targets 29); with ask set,
// Android asks the user for it if it is not granted.
int ble_permission(int ask) {
	if (sdk < 31) {
		return 1; // BLUETOOTH, granted at install
	}
	int attached;
	JNIEnv *e = env(&attached);
	jclass ctxClass = (*e)->GetObjectClass(e, ctx);
	jstring perm = (*e)->NewStringUTF(e, "android.permission.BLUETOOTH_CONNECT");
	jint granted = (*e)->CallIntMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "checkSelfPermission", "(Ljava/lang/String;)I"), perm);
	int ok = !failed(e) && granted == 0;
	if (!ok && ask) {
		jobjectArray perms = (*e)->NewObjectArray(e, 1, (*e)->FindClass(e, "java/lang/String"), perm);
		(*e)->CallVoidMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "requestPermissions", "([Ljava/lang/String;I)V"), perms, 7);
		failed(e);
	}
	release(attached);
	return ok;
}

// ble_connect starts connecting to the device at mac ("FA:BA:25:00:0F:4E")
// for the Go connection h: a global reference to its BluetoothGatt, or 0.
uintptr_t ble_connect(jlong h, const char *mac) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject gatt = NULL;
	jclass ctxClass = (*e)->GetObjectClass(e, ctx);
	jstring service = (*e)->NewStringUTF(e, "bluetooth");
	jobject manager = (*e)->CallObjectMethod(e, ctx, (*e)->GetMethodID(e, ctxClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;"), service);
	if (failed(e) || manager == NULL) {
		LOG("connect: no Bluetooth manager");
		goto out;
	}
	jclass managerClass = (*e)->GetObjectClass(e, manager);
	jobject adapter = (*e)->CallObjectMethod(e, manager, (*e)->GetMethodID(e, managerClass, "getAdapter", "()Landroid/bluetooth/BluetoothAdapter;"));
	if (failed(e) || adapter == NULL) {
		LOG("connect: no Bluetooth adapter");
		goto out;
	}
	jclass adapterClass = (*e)->GetObjectClass(e, adapter);
	jobject device = (*e)->CallObjectMethod(e, adapter, (*e)->GetMethodID(e, adapterClass, "getRemoteDevice", "(Ljava/lang/String;)Landroid/bluetooth/BluetoothDevice;"), (*e)->NewStringUTF(e, mac));
	if (failed(e) || device == NULL) {
		LOG("connect: no device %s", mac);
		goto out;
	}
	jobject cb = (*e)->NewObject(e, bridge, (*e)->GetMethodID(e, bridge, "<init>", "(J)V"), h);
	if (failed(e) || cb == NULL) {
		LOG("connect: no BleBridge");
		goto out;
	}
	jclass deviceClass = (*e)->GetObjectClass(e, device);
	jobject g;
	if (sdk >= 23) {
		// TRANSPORT_LE: the remote is a Bluetooth Low Energy device.
		g = (*e)->CallObjectMethod(e, device, (*e)->GetMethodID(e, deviceClass, "connectGatt", "(Landroid/content/Context;ZLandroid/bluetooth/BluetoothGattCallback;I)Landroid/bluetooth/BluetoothGatt;"), ctx, JNI_FALSE, cb, 2);
	} else {
		g = (*e)->CallObjectMethod(e, device, (*e)->GetMethodID(e, deviceClass, "connectGatt", "(Landroid/content/Context;ZLandroid/bluetooth/BluetoothGattCallback;)Landroid/bluetooth/BluetoothGatt;"), ctx, JNI_FALSE, cb);
	}
	if (failed(e) || g == NULL) {
		LOG("connect: connectGatt to %s gave nothing (sdk %d, target %d, bridge %p)", mac, sdk, target, (void *)bridge);
		goto out;
	}
	LOG("connect: connecting to %s", mac);
	gatt = (*e)->NewGlobalRef(e, g);
out:
	release(attached);
	return (uintptr_t)gatt;
}

// call runs a no-argument boolean method of gatt.
static int gattBool(const char *name, uintptr_t gatt) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt;
	jboolean ok = (*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, (*e)->GetObjectClass(e, g), name, "()Z"));
	int r = !failed(e) && ok;
	release(attached);
	return r;
}

int ble_discover(uintptr_t gatt) { return gattBool("discoverServices", gatt); }

// ble_tune asks for a fast link, as the app does: high priority, and the
// 2M PHY from Android 8.
void ble_tune(uintptr_t gatt) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt;
	jclass gc = (*e)->GetObjectClass(e, g);
	(*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, gc, "requestConnectionPriority", "(I)Z"), 1);
	failed(e);
	if (sdk >= 26) {
		(*e)->CallVoidMethod(e, g, (*e)->GetMethodID(e, gc, "setPreferredPhy", "(III)V"), 2, 2, 0);
		failed(e);
	}
	release(attached);
}

int ble_mtu(uintptr_t gatt, int mtu) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt;
	jboolean ok = (*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, (*e)->GetObjectClass(e, g), "requestMtu", "(I)Z"), mtu);
	int r = !failed(e) && ok;
	release(attached);
	return r;
}

static jobject uuid(JNIEnv *e, const char *s) {
	jclass c = (*e)->FindClass(e, "java/util/UUID");
	return (*e)->CallStaticObjectMethod(e, c, (*e)->GetStaticMethodID(e, c, "fromString", "(Ljava/lang/String;)Ljava/util/UUID;"), (*e)->NewStringUTF(e, s));
}

// ble_characteristic is a global reference to the characteristic of the
// service on gatt, once discovered; 0 if it has none.
uintptr_t ble_characteristic(uintptr_t gatt, const char *svc, const char *chr) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt, out = NULL;
	jobject s = (*e)->CallObjectMethod(e, g, (*e)->GetMethodID(e, (*e)->GetObjectClass(e, g), "getService", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothGattService;"), uuid(e, svc));
	if (!failed(e) && s != NULL) {
		jobject c = (*e)->CallObjectMethod(e, s, (*e)->GetMethodID(e, (*e)->GetObjectClass(e, s), "getCharacteristic", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothGattCharacteristic;"), uuid(e, chr));
		if (!failed(e) && c != NULL) {
			out = (*e)->NewGlobalRef(e, c);
		}
	}
	release(attached);
	return (uintptr_t)out;
}

// ble_notify turns the characteristic's notifications on: locally, then
// in its client configuration descriptor, which onDescriptor answers.
int ble_notify(uintptr_t gatt, uintptr_t chr) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt, c = (jobject)chr;
	jclass gc = (*e)->GetObjectClass(e, g), cc = (*e)->GetObjectClass(e, c);
	int r = 0;
	jboolean ok = (*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, gc, "setCharacteristicNotification", "(Landroid/bluetooth/BluetoothGattCharacteristic;Z)Z"), c, JNI_TRUE);
	if (failed(e) || !ok) {
		goto out;
	}
	jobject d = (*e)->CallObjectMethod(e, c, (*e)->GetMethodID(e, cc, "getDescriptor", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothGattDescriptor;"), uuid(e, "00002902-0000-1000-8000-00805f9b34fb"));
	if (failed(e) || d == NULL) {
		goto out;
	}
	jbyteArray on = (*e)->NewByteArray(e, 2);
	jbyte v[2] = {1, 0};
	(*e)->SetByteArrayRegion(e, on, 0, 2, v);
	if (sdk >= 33) {
		jint st = (*e)->CallIntMethod(e, g, (*e)->GetMethodID(e, gc, "writeDescriptor", "(Landroid/bluetooth/BluetoothGattDescriptor;[B)I"), d, on);
		r = !failed(e) && st == 0;
	} else {
		jclass dc = (*e)->GetObjectClass(e, d);
		(*e)->CallBooleanMethod(e, d, (*e)->GetMethodID(e, dc, "setValue", "([B)Z"), on);
		jboolean w = (*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, gc, "writeDescriptor", "(Landroid/bluetooth/BluetoothGattDescriptor;)Z"), d);
		r = !failed(e) && w;
	}
out:
	release(attached);
	return r;
}

// ble_write writes value to the characteristic without response; the
// next write waits for onWrite.
int ble_write(uintptr_t gatt, uintptr_t chr, void *value, int n) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt, c = (jobject)chr;
	jclass gc = (*e)->GetObjectClass(e, g);
	jbyteArray b = (*e)->NewByteArray(e, n);
	(*e)->SetByteArrayRegion(e, b, 0, n, value);
	int r;
	if (sdk >= 33) {
		jint st = (*e)->CallIntMethod(e, g, (*e)->GetMethodID(e, gc, "writeCharacteristic", "(Landroid/bluetooth/BluetoothGattCharacteristic;[BI)I"), c, b, 1);
		r = failed(e) ? -1 : st;
	} else {
		jclass cc = (*e)->GetObjectClass(e, c);
		(*e)->CallVoidMethod(e, c, (*e)->GetMethodID(e, cc, "setWriteType", "(I)V"), 1);
		(*e)->CallBooleanMethod(e, c, (*e)->GetMethodID(e, cc, "setValue", "([B)Z"), b);
		jboolean ok = (*e)->CallBooleanMethod(e, g, (*e)->GetMethodID(e, gc, "writeCharacteristic", "(Landroid/bluetooth/BluetoothGattCharacteristic;)Z"), c);
		r = failed(e) ? -1 : (ok ? 0 : 1);
	}
	(*e)->DeleteLocalRef(e, b);
	release(attached);
	return r;
}

// ble_close disconnects and lets go of gatt and chr.
void ble_close(uintptr_t gatt, uintptr_t chr) {
	int attached;
	JNIEnv *e = env(&attached);
	jobject g = (jobject)gatt;
	if (g != NULL) {
		jclass gc = (*e)->GetObjectClass(e, g);
		(*e)->CallVoidMethod(e, g, (*e)->GetMethodID(e, gc, "disconnect", "()V"));
		failed(e);
		(*e)->CallVoidMethod(e, g, (*e)->GetMethodID(e, gc, "close", "()V"));
		failed(e);
		(*e)->DeleteGlobalRef(e, g);
	}
	if (chr != 0) {
		(*e)->DeleteGlobalRef(e, (jobject)chr);
	}
	release(attached);
}
