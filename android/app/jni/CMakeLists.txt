cmake_minimum_required(VERSION 3.22)

project(TaktChaseProbe C)

# SDL is fetched at a pinned commit by android/scripts/fetch-sdl.sh (see
# android/SDL_VERSION). It is not vendored in this repository.
if(NOT EXISTS "${CMAKE_CURRENT_SOURCE_DIR}/SDL/CMakeLists.txt")
    message(FATAL_ERROR
        "SDL sources not found in ${CMAKE_CURRENT_SOURCE_DIR}/SDL.\n"
        "Run android/scripts/fetch-sdl.sh first.")
endif()

# Directory (per ABI) where the Go c-shared library is placed.
set(TAKT_GO_LIB_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../src/main/jniLibs/${ANDROID_ABI}")

# Only the shared library is needed on Android; skip test programs and static.
set(SDL_SHARED ON CACHE BOOL "" FORCE)
set(SDL_STATIC OFF CACHE BOOL "" FORCE)
set(SDL_TEST_LIBRARY OFF CACHE BOOL "" FORCE)
set(SDL_TESTS OFF CACHE BOOL "" FORCE)

add_subdirectory(SDL)
add_subdirectory(src)
