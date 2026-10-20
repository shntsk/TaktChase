/* TaktChase Android feasibility prototype (stage 2).
 *
 * Deliberately minimal: SDL3 + OpenGL ES 3.x context, one clear colour and one
 * triangle drawn with a GLSL ES 3.00 shader, plus a call into the Go library.
 * It does NOT start the launcher or any gameplay code. */
#include <SDL3/SDL.h>
#include <SDL3/SDL_main.h>

#include <GLES3/gl3.h>
#include <math.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include "go_bridge.h"
#include "takt_log.h"
#include "taktchase_sprite_gles.h"

/* Clear colour (teal) as 8-bit values, used for the self-check below. */
#define CLEAR_R 13
#define CLEAR_G 140
#define CLEAR_B 166

/* Stage 2's diagnostic triangle remains as a bridge/GLES smoke fixture.
 * These shaders are deliberately not used for the game sprite-shader test. */
static const char *kProbeVertexSrc =
    "#version 300 es\n"
    "layout(location = 0) in vec2 aPos;\n"
    "layout(location = 1) in vec3 aCol;\n"
    "uniform float uAngle;\n"
    "out vec3 vCol;\n"
    "void main() {\n"
    "    float c = cos(uAngle), s = sin(uAngle);\n"
    "    vec2 p = vec2(c * aPos.x - s * aPos.y, s * aPos.x + c * aPos.y);\n"
    "    gl_Position = vec4(p, 0.0, 1.0);\n"
    "    vCol = aCol;\n"
    "}\n";

static const char *kProbeFragmentSrc =
    "#version 300 es\n"
    "precision mediump float;\n"
    "in vec3 vCol;\n"
    "out vec4 outColor;\n"
    "void main() { outColor = vec4(vCol, 1.0); }\n";

typedef struct {
    GLuint program;
    GLuint vao;
    GLuint vbo;
    GLint angleLoc;
    bool ok;
} Scene;

static GLuint compile_shader(GLenum type, const char *src) {
    GLuint sh = glCreateShader(type);
    glShaderSource(sh, 1, &src, NULL);
    glCompileShader(sh);
    GLint ok = GL_FALSE;
    glGetShaderiv(sh, GL_COMPILE_STATUS, &ok);
    if (!ok) {
        char log[512] = {0};
        glGetShaderInfoLog(sh, (GLsizei)sizeof(log) - 1, NULL, log);
        TLOGE("TAKT_PROBE shader_compile=FAIL type=0x%x log=%s", (unsigned)type, log);
        glDeleteShader(sh);
        return 0;
    }
    return sh;
}

static void scene_init(Scene *sc) {
    memset(sc, 0, sizeof(*sc));
    GLuint vs = compile_shader(GL_VERTEX_SHADER, kProbeVertexSrc);
    GLuint fs = compile_shader(GL_FRAGMENT_SHADER, kProbeFragmentSrc);
    if (!vs || !fs) {
        if (vs) glDeleteShader(vs);
        if (fs) glDeleteShader(fs);
        return;
    }
    sc->program = glCreateProgram();
    glAttachShader(sc->program, vs);
    glAttachShader(sc->program, fs);
    glLinkProgram(sc->program);
    glDeleteShader(vs);
    glDeleteShader(fs);
    GLint ok = GL_FALSE;
    glGetProgramiv(sc->program, GL_LINK_STATUS, &ok);
    if (!ok) {
        char log[512] = {0};
        glGetProgramInfoLog(sc->program, (GLsizei)sizeof(log) - 1, NULL, log);
        TLOGE("TAKT_PROBE program_link=FAIL log=%s", log);
        glDeleteProgram(sc->program);
        sc->program = 0;
        return;
    }
    sc->angleLoc = glGetUniformLocation(sc->program, "uAngle");

    /* Triangle whose centroid is the origin, so it covers the screen centre. */
    static const GLfloat verts[] = {
        /* x      y      r     g     b */
        0.00f,  0.60f, 1.0f, 0.0f, 0.0f,
       -0.52f, -0.30f, 0.0f, 1.0f, 0.0f,
        0.52f, -0.30f, 0.0f, 0.0f, 1.0f,
    };
    glGenVertexArrays(1, &sc->vao);
    glGenBuffers(1, &sc->vbo);
    glBindVertexArray(sc->vao);
    glBindBuffer(GL_ARRAY_BUFFER, sc->vbo);
    glBufferData(GL_ARRAY_BUFFER, (GLsizeiptr)sizeof(verts), verts, GL_STATIC_DRAW);
    glEnableVertexAttribArray(0);
    glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, 5 * sizeof(GLfloat), (const void *)0);
    glEnableVertexAttribArray(1);
    glVertexAttribPointer(1, 3, GL_FLOAT, GL_FALSE, 5 * sizeof(GLfloat),
                          (const void *)(2 * sizeof(GLfloat)));
    glBindVertexArray(0);

    sc->ok = glGetError() == GL_NO_ERROR;
    TLOGI("TAKT_PROBE scene_init=%s", sc->ok ? "ok" : "FAIL (GL error)");
}

/* Compile/link the real game's sprite shaders after the GLES context is current.
 * This is a shader compatibility checkpoint, not a claim that the full batch
 * renderer or gameplay scene has been ported. */
static bool sprite_shader_selftest(void) {
    GLuint vs = compile_shader(GL_VERTEX_SHADER, kSpriteVertexGlesSrc);
    GLuint fs = compile_shader(GL_FRAGMENT_SHADER, kSpriteFragmentGlesSrc);
    if (!vs || !fs) {
        if (vs) glDeleteShader(vs);
        if (fs) glDeleteShader(fs);
        TLOGE("TAKT_RENDER sprite_shader_compile=FAIL stage=compile");
        return false;
    }

    GLuint program = glCreateProgram();
    glAttachShader(program, vs);
    glAttachShader(program, fs);
    glLinkProgram(program);
    glDeleteShader(vs);
    glDeleteShader(fs);

    GLint linked = GL_FALSE;
    glGetProgramiv(program, GL_LINK_STATUS, &linked);
    if (linked != GL_TRUE) {
        char log[1024] = {0};
        glGetProgramInfoLog(program, (GLsizei)sizeof(log) - 1, NULL, log);
        TLOGE("TAKT_RENDER sprite_shader_compile=FAIL stage=link log=%s", log);
        glDeleteProgram(program);
        return false;
    }

    static const char *required_attributes[] = {
        "base_pos", "base_uv", "in_origin", "in_scale", "in_position",
        "in_rotation", "in_u", "in_v", "in_layer", "in_color", "in_additive"
    };
    bool interface_ok = true;
    for (size_t i = 0; i < sizeof(required_attributes) / sizeof(required_attributes[0]); ++i) {
        if (glGetAttribLocation(program, required_attributes[i]) < 0) {
            TLOGE("TAKT_RENDER sprite_shader_interface=FAIL missing_attribute=%s",
                  required_attributes[i]);
            interface_ok = false;
        }
    }
    if (glGetUniformLocation(program, "proj") < 0 ||
        glGetUniformLocation(program, "tex") < 0) {
        TLOGE("TAKT_RENDER sprite_shader_interface=FAIL missing_uniform=proj_or_tex");
        interface_ok = false;
    }

    glDeleteProgram(program);
    if (!interface_ok) {
        TLOGE("TAKT_RENDER sprite_shader_compile=FAIL stage=interface");
        return false;
    }
    TLOGI("TAKT_RENDER sprite_shader_compile=PASS source=assets/shaders/sprite.vsh+sprite.fsh "
          "target=GLES3 attributes=11 uniforms=proj,tex");
    return true;
}

static void scene_destroy(Scene *sc) {
    if (sc->vbo) glDeleteBuffers(1, &sc->vbo);
    if (sc->vao) glDeleteVertexArrays(1, &sc->vao);
    if (sc->program) glDeleteProgram(sc->program);
    memset(sc, 0, sizeof(*sc));
}

static void scene_draw(const Scene *sc, int w, int h, float angle) {
    glViewport(0, 0, w, h);
    glClearColor(CLEAR_R / 255.0f, CLEAR_G / 255.0f, CLEAR_B / 255.0f, 1.0f);
    glClear(GL_COLOR_BUFFER_BIT);
    if (!sc->ok) return;
    glUseProgram(sc->program);
    glUniform1f(sc->angleLoc, angle);
    glBindVertexArray(sc->vao);
    glDrawArrays(GL_TRIANGLES, 0, 3);
    glBindVertexArray(0);
}

/* Reads back two pixels of the frame that was just drawn: a corner must still
 * have the clear colour and the centre must be covered by the triangle. */
static bool scene_selfcheck(int w, int h) {
    unsigned char corner[4] = {0}, centre[4] = {0};
    glReadPixels(2, 2, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, corner);
    glReadPixels(w / 2, h / 2, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, centre);
    bool corner_ok = abs(corner[0] - CLEAR_R) <= 2 && abs(corner[1] - CLEAR_G) <= 2 &&
                     abs(corner[2] - CLEAR_B) <= 2;
    int dist = abs(centre[0] - CLEAR_R) + abs(centre[1] - CLEAR_G) + abs(centre[2] - CLEAR_B);
    bool centre_ok = dist > 60;
    TLOGI("TAKT_PROBE pixel_corner=%u,%u,%u pixel_centre=%u,%u,%u", corner[0], corner[1],
          corner[2], centre[0], centre[1], centre[2]);
    TLOGI("TAKT_PROBE scene_check=%s", (corner_ok && centre_ok) ? "PASS" : "FAIL");
    return corner_ok && centre_ok;
}

static bool create_gles_context(SDL_Window **out_win, SDL_GLContext *out_ctx) {
    SDL_GL_SetAttribute(SDL_GL_CONTEXT_PROFILE_MASK, SDL_GL_CONTEXT_PROFILE_ES);
    SDL_GL_SetAttribute(SDL_GL_CONTEXT_MAJOR_VERSION, 3);
    SDL_GL_SetAttribute(SDL_GL_CONTEXT_MINOR_VERSION, 0);
    SDL_GL_SetAttribute(SDL_GL_RED_SIZE, 8);
    SDL_GL_SetAttribute(SDL_GL_GREEN_SIZE, 8);
    SDL_GL_SetAttribute(SDL_GL_BLUE_SIZE, 8);
    SDL_GL_SetAttribute(SDL_GL_DOUBLEBUFFER, 1);

    /* Android ignores the requested size; the window is always fullscreen. */
    SDL_Window *win = SDL_CreateWindow("TaktChase probe", 1280, 720,
                                       SDL_WINDOW_OPENGL | SDL_WINDOW_FULLSCREEN);
    if (!win) {
        TLOGE("TAKT_PROBE window_create=FAIL err=%s", SDL_GetError());
        return false;
    }
    SDL_GLContext ctx = SDL_GL_CreateContext(win);
    if (!ctx) {
        TLOGE("TAKT_PROBE gl_context=FAIL err=%s", SDL_GetError());
        SDL_DestroyWindow(win);
        return false;
    }
    *out_win = win;
    *out_ctx = ctx;
    return true;
}

int main(int argc, char *argv[]) {
    (void)argc;
    (void)argv;

    int linked = SDL_GetVersion();
    TLOGI("TAKT_PROBE main_start sdl_compiled=%d.%d.%d sdl_linked=%d.%d.%d", SDL_MAJOR_VERSION,
          SDL_MINOR_VERSION, SDL_MICRO_VERSION, SDL_VERSIONNUM_MAJOR(linked),
          SDL_VERSIONNUM_MINOR(linked), SDL_VERSIONNUM_MICRO(linked));

    if (!SDL_Init(SDL_INIT_VIDEO)) {
        TLOGE("TAKT_PROBE sdl_init=FAIL err=%s", SDL_GetError());
        return 1;
    }
    TLOGI("TAKT_PROBE sdl_init=ok video_driver=%s", SDL_GetCurrentVideoDriver());

    /* Go call from the SDL_main thread (pure C ABI, no JNI involved). */
    bool go_ok = takt_go_selftest() != 0;

    SDL_Window *window = NULL;
    SDL_GLContext ctx = NULL;
    if (!create_gles_context(&window, &ctx)) {
        SDL_Quit();
        return 1;
    }

    SDL_GL_SetSwapInterval(1);

    GLint major = 0, minor = 0;
    glGetIntegerv(GL_MAJOR_VERSION, &major);
    glGetIntegerv(GL_MINOR_VERSION, &minor);
    TLOGI("TAKT_PROBE gl_version_string=%s", (const char *)glGetString(GL_VERSION));
    TLOGI("TAKT_PROBE gl_renderer=%s", (const char *)glGetString(GL_RENDERER));
    TLOGI("TAKT_PROBE gl_vendor=%s", (const char *)glGetString(GL_VENDOR));
    TLOGI("TAKT_PROBE gl_glsl=%s", (const char *)glGetString(GL_SHADING_LANGUAGE_VERSION));
    TLOGI("TAKT_PROBE gles_major_minor=%d.%d", (int)major, (int)minor);
    if (major < 3) {
        TLOGE("TAKT_PROBE gles3=FAIL (got %d.%d)", (int)major, (int)minor);
        SDL_GL_DestroyContext(ctx);
        SDL_DestroyWindow(window);
        SDL_Quit();
        return 1;
    }

    if (!sprite_shader_selftest()) {
        SDL_GL_DestroyContext(ctx);
        SDL_DestroyWindow(window);
        SDL_Quit();
        return 1;
    }

    Scene scene;
    scene_init(&scene);

    bool running = true;
    bool active = true;
    bool checked = false;
    bool background_seen = false;
    bool check_resume_frame = false;
    Uint64 frames = 0;
    Uint64 t0 = SDL_GetTicks();

    while (running) {
        SDL_Event ev;
        bool have_event = active ? SDL_PollEvent(&ev) : SDL_WaitEventTimeout(&ev, 100);
        while (have_event) {
            switch (ev.type) {
            case SDL_EVENT_QUIT:
                TLOGI("TAKT_PROBE event=QUIT");
                running = false;
                break;
            case SDL_EVENT_TERMINATING:
                TLOGI("TAKT_PROBE event=TERMINATING");
                running = false;
                break;
            case SDL_EVENT_LOW_MEMORY:
                TLOGW("TAKT_PROBE event=LOW_MEMORY");
                break;
            case SDL_EVENT_WILL_ENTER_BACKGROUND:
                TLOGI("TAKT_PROBE event=WILL_ENTER_BACKGROUND (stop drawing)");
                active = false;
                break;
            case SDL_EVENT_DID_ENTER_BACKGROUND:
                TLOGI("TAKT_PROBE event=DID_ENTER_BACKGROUND");
                background_seen = true;
                active = false;
                break;
            case SDL_EVENT_WILL_ENTER_FOREGROUND:
                TLOGI("TAKT_PROBE event=WILL_ENTER_FOREGROUND");
                break;
            case SDL_EVENT_DID_ENTER_FOREGROUND:
                TLOGI("TAKT_PROBE event=DID_ENTER_FOREGROUND (resume drawing)");
                active = true;
                check_resume_frame = background_seen;
                break;
            case SDL_EVENT_WINDOW_PIXEL_SIZE_CHANGED:
                TLOGI("TAKT_PROBE event=PIXEL_SIZE_CHANGED %dx%d", (int)ev.window.data1,
                      (int)ev.window.data2);
                break;
            case SDL_EVENT_FINGER_DOWN:
                TLOGI("TAKT_PROBE event=FINGER_DOWN x=%.2f y=%.2f", ev.tfinger.x, ev.tfinger.y);
                break;
            default:
                break;
            }
            have_event = SDL_PollEvent(&ev);
        }
        if (!running) break;
        if (!active) continue;

        int w = 0, h = 0;
        SDL_GetWindowSizeInPixels(window, &w, &h);
        float angle = (float)(SDL_GetTicks() - t0) / 1000.0f;
        scene_draw(&scene, w, h, angle);

        if (check_resume_frame) {
            /* Verify a real post-resume frame by reading the rendered pixels,
             * not merely by checking that the Activity process survived. */
            bool frame_ok = scene.ok && scene_selfcheck(w, h);
            GLenum resume_error = glGetError();
            bool resumed_ok = frame_ok && resume_error == GL_NO_ERROR;
            TLOGI("TAKT_PROBE resumed_render=%s gl_error=0x%x",
                  resumed_ok ? "PASS" : "FAIL", (unsigned)resume_error);
            check_resume_frame = false;
            if (!resumed_ok) running = false;
        }

        if (!checked) {
            /* Only the first frame is read back; glReadPixels stalls the GPU. */
            checked = true;
            bool pass = scene.ok && scene_selfcheck(w, h);
            GLenum err = glGetError();
            TLOGI("TAKT_PROBE first_frame size=%dx%d gl_error=0x%x scene=%s go=%s", w, h,
                  (unsigned)err, pass ? "PASS" : "FAIL", go_ok ? "PASS" : "FAIL");
        }

        SDL_GL_SwapWindow(window);
        frames++;
        if (frames % 300 == 0) {
            TLOGI("TAKT_PROBE heartbeat frames=%llu", (unsigned long long)frames);
        }
    }

    TLOGI("TAKT_PROBE shutdown frames=%llu", (unsigned long long)frames);
    scene_destroy(&scene);
    SDL_GL_DestroyContext(ctx);
    SDL_DestroyWindow(window);
    SDL_Quit();
    return 0;
}
