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

# Compile the actual game's sprite shaders through the GLES 3.00 compiler at
# runtime. The generated header is derived from the desktop sources so they
# cannot quietly drift into a disconnected hand-written demo shader.
find_package(Python3 REQUIRED COMPONENTS Interpreter)
get_filename_component(TAKTCHASE_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../../" ABSOLUTE)
set(TAKTCHASE_GLES_SPRITE_HEADER "${CMAKE_CURRENT_BINARY_DIR}/taktchase_sprite_gles.h")
add_custom_command(
    OUTPUT "${TAKTCHASE_GLES_SPRITE_HEADER}"
    COMMAND "${Python3_EXECUTABLE}"
            "${TAKTCHASE_ROOT}/android/scripts/generate-gles-sprite-header.py"
            "${TAKTCHASE_GLES_SPRITE_HEADER}"
    DEPENDS
        "${TAKTCHASE_ROOT}/android/scripts/generate-gles-sprite-header.py"
        "${TAKTCHASE_ROOT}/assets/shaders/sprite.vsh"
        "${TAKTCHASE_ROOT}/assets/shaders/sprite.fsh"
    COMMENT "Adapting TaktChase sprite shaders to GLSL ES 3.00 for runtime verification"
    VERBATIM
)
set_source_files_properties("${TAKTCHASE_GLES_SPRITE_HEADER}"
    PROPERTIES GENERATED TRUE HEADER_FILE_ONLY TRUE)

add_library(taktgo SHARED IMPORTED)
set_target_properties(taktgo PROPERTIES
    IMPORTED_LOCATION "${TAKT_GO_LIB}"
    IMPORTED_SONAME "libtaktgo.so")

# SDL's Java side loads "main" and calls SDL_main from it.
add_library(main SHARED
    taktchase_main.c
    go_bridge.c
    "${TAKTCHASE_GLES_SPRITE_HEADER}"
)
target_include_directories(main PRIVATE "${CMAKE_CURRENT_BINARY_DIR}")
target_link_libraries(main PRIVATE SDL3::SDL3 taktgo GLESv3 log m)
target_compile_options(main PRIVATE -Wall -Wextra)
