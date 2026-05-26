//go:build linux && (native_import || native_call || native_unload || native_module || debug)

package native_module

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>

typedef char* (*native_entry_t)(int, char**);

static void* native_dlopen(char* filePath, char** errorOut) {
	dlerror();
	void* handle = dlopen(filePath, RTLD_NOW | RTLD_LOCAL);
	if (handle == NULL) {
		char* err = dlerror();
		*errorOut = err != NULL ? err : "dlopen failed";
	}
	return handle;
}

static char* native_call(void* handle, char* functionName, int argc, char** argv, char** errorOut) {
	dlerror();
	void* symbol = dlsym(handle, functionName);
	char* err = dlerror();
	if (err != NULL) {
		*errorOut = err;
		return NULL;
	}
	return ((native_entry_t)symbol)(argc, argv);
}

static int native_dlclose(void* handle, char** errorOut) {
	dlerror();
	int result = dlclose(handle);
	if (result != 0) {
		char* err = dlerror();
		*errorOut = err != NULL ? err : "dlclose failed";
	}
	return result;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func loadNativeModule(fileID string, fileBytes []byte) (nativeModule, error) {
	if module, err := loadNativeModuleFromMemfd(fileID, fileBytes); err == nil {
		return module, nil
	}
	return loadNativeModuleFromTemp(fileBytes)
}

func loadNativeModuleFromMemfd(fileID string, fileBytes []byte) (nativeModule, error) {
	fd, err := unix.MemfdCreate("poseidon-native-"+fileID, unix.MFD_CLOEXEC)
	if err != nil {
		return nativeModule{}, err
	}
	defer unix.Close(fd)

	if err := writeAllFD(fd, fileBytes); err != nil {
		return nativeModule{}, err
	}

	path := fmt.Sprintf("/proc/self/fd/%d", fd)
	handle, err := dlopenPath(path)
	if err != nil {
		return nativeModule{}, err
	}
	return nativeModule{handle: handle}, nil
}

func loadNativeModuleFromTemp(fileBytes []byte) (nativeModule, error) {
	file, err := os.CreateTemp("", "poseidon-native-*.so")
	if err != nil {
		return nativeModule{}, err
	}
	path := file.Name()
	if _, err = file.Write(fileBytes); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nativeModule{}, err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(path)
		return nativeModule{}, err
	}

	handle, err := dlopenPath(path)
	if err != nil {
		_ = os.Remove(path)
		return nativeModule{}, err
	}
	return nativeModule{handle: handle, path: path}, nil
}

func writeAllFD(fd int, fileBytes []byte) error {
	for len(fileBytes) > 0 {
		n, err := unix.Write(fd, fileBytes)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		fileBytes = fileBytes[n:]
	}
	_, err := unix.Seek(fd, 0, io.SeekStart)
	return err
}

func dlopenPath(path string) (uintptr, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var cErr *C.char
	handle := C.native_dlopen(cPath, &cErr)
	if handle == nil {
		if cErr != nil {
			return 0, errors.New(C.GoString(cErr))
		}
		return 0, fmt.Errorf("dlopen failed")
	}
	return uintptr(handle), nil
}

func callNativeModule(module nativeModule, functionName string, args []string) (string, error) {
	cFunctionName := C.CString(functionName)
	defer C.free(unsafe.Pointer(cFunctionName))

	cArgc := C.int(len(args) + 1)
	cArgs := make([]*C.char, len(args)+2)
	cArgs[0] = C.CString(os.Args[0])
	for i, arg := range args {
		cArgs[i+1] = C.CString(arg)
	}
	for _, cArg := range cArgs {
		if cArg != nil {
			defer C.free(unsafe.Pointer(cArg))
		}
	}

	var cErr *C.char
	cArgv := (**C.char)(unsafe.Pointer(&cArgs[0]))
	result := C.native_call(unsafe.Pointer(module.handle), cFunctionName, cArgc, cArgv, &cErr)
	if cErr != nil {
		return "", errors.New(C.GoString(cErr))
	}
	if result == nil {
		return "", nil
	}
	return C.GoString(result), nil
}

func closeNativeModule(module nativeModule) error {
	var cErr *C.char
	result := C.native_dlclose(unsafe.Pointer(module.handle), &cErr)
	if result != 0 {
		if cErr != nil {
			return errors.New(C.GoString(cErr))
		}
		return fmt.Errorf("dlclose failed")
	}
	return nil
}

func cleanupNativeModule(module nativeModule) {
	if module.path != "" {
		_ = os.Remove(module.path)
	}
}
