cmake_minimum_required(VERSION 3.22)

if(NOT TARGET SDL3::SDL3)
    find_package(SDL3 CONFIG REQUIRED)
endif()

# libtaktgo.so is produced by android/scripts/build-go.sh (Go, c-shared) and
# placed in src/main/jniLibs/<abi>/ so Gradle packages it. It is only linked
# here, never rebuilt by CMake.
set(TAKT_GO_LIB "${TAKT_GO_LIB_DIR}/libtaktgo.so")
if(NOT EXISTS "${TAKT_GO_LIB}")
    message(FATAL_ERROR
        "Go library not found: ${TAKT_GO_LIB}\n"
        "Run android/scripts/build-go.sh for ABI ${ANDROID_ABI} before building the APK.")
endif()

add_library(taktgo SHARED IMPORTED)
set_target_properties(taktgo PROPERTIES
    IMPORTED_LOCATION "${TAKT_GO_LIB}"
    IMPORTED_SONAME "libtaktgo.so")

# SDL's Java side loads "main" and calls SDL_main from it.
add_library(main SHARED
    taktchase_main.c
    go_bridge.c
)

target_link_libraries(main PRIVATE SDL3::SDL3 taktgo GLESv3 log m)
target_compile_options(main PRIVATE -Wall -Wextra)
