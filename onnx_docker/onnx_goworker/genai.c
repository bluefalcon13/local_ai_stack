#include "genai.h"
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>

static void* lib_handle = NULL;
static char last_error[1024] = {0};

typedef OgaResult* (*fn_OgaCreateModel)(const char* config_path, OgaModel** out);
typedef void (*fn_OgaDestroyModel)(OgaModel* model);
typedef OgaResult* (*fn_OgaCreateTokenizer)(const OgaModel* model, OgaTokenizer** out);
typedef void (*fn_OgaDestroyTokenizer)(OgaTokenizer* tokenizer);
typedef OgaResult* (*fn_OgaCreateTokenizerStream)(const OgaTokenizer* tokenizer, OgaTokenizerStream** out);
typedef void (*fn_OgaDestroyTokenizerStream)(OgaTokenizerStream* stream);
typedef OgaResult* (*fn_OgaTokenizerStreamDecode)(OgaTokenizerStream* stream, int32_t token, const char** out);
typedef OgaResult* (*fn_OgaCreateSequences)(OgaSequences** out);
typedef void (*fn_OgaDestroySequences)(OgaSequences* sequences);
typedef size_t (*fn_OgaSequencesGetSequenceCount)(const OgaSequences* sequences, size_t sequence_index);
typedef const int32_t* (*fn_OgaSequencesGetSequenceData)(const OgaSequences* sequences, size_t sequence_index);
typedef OgaResult* (*fn_OgaTokenizerEncode)(const OgaTokenizer* tokenizer, const char* str, OgaSequences* sequences);
typedef OgaResult* (*fn_OgaCreateGeneratorParams)(const OgaModel* model, OgaGeneratorParams** out);
typedef void (*fn_OgaDestroyGeneratorParams)(OgaGeneratorParams* params);
typedef OgaResult* (*fn_OgaGeneratorParamsSetSearchNumber)(OgaGeneratorParams* params, const char* name, double value);
typedef OgaResult* (*fn_OgaGeneratorParamsSetSearchBool)(OgaGeneratorParams* params, const char* name, bool value);
typedef OgaResult* (*fn_OgaCreateGenerator)(const OgaModel* model, const OgaGeneratorParams* params, OgaGenerator** out);
typedef void (*fn_OgaDestroyGenerator)(OgaGenerator* generator);
typedef OgaResult* (*fn_OgaGenerator_AppendTokenSequences)(OgaGenerator* generator, const OgaSequences* sequences);
typedef bool (*fn_OgaGenerator_IsDone)(OgaGenerator* generator);
typedef OgaResult* (*fn_OgaGenerator_GenerateNextToken)(OgaGenerator* generator);
typedef OgaResult* (*fn_OgaGenerator_GetNextTokens)(const OgaGenerator* generator, const int32_t** out, size_t* out_count);
typedef const char* (*fn_OgaResultGetError)(const OgaResult* result);
typedef void (*fn_OgaDestroyResult)(OgaResult* result);

static fn_OgaCreateModel p_OgaCreateModel = NULL;
static fn_OgaDestroyModel p_OgaDestroyModel = NULL;
static fn_OgaCreateTokenizer p_OgaCreateTokenizer = NULL;
static fn_OgaDestroyTokenizer p_OgaDestroyTokenizer = NULL;
static fn_OgaCreateTokenizerStream p_OgaCreateTokenizerStream = NULL;
static fn_OgaDestroyTokenizerStream p_OgaDestroyTokenizerStream = NULL;
static fn_OgaTokenizerStreamDecode p_OgaTokenizerStreamDecode = NULL;
static fn_OgaCreateSequences p_OgaCreateSequences = NULL;
static fn_OgaDestroySequences p_OgaDestroySequences = NULL;
static fn_OgaSequencesGetSequenceCount p_OgaSequencesGetSequenceCount = NULL;
static fn_OgaSequencesGetSequenceData p_OgaSequencesGetSequenceData = NULL;
static fn_OgaTokenizerEncode p_OgaTokenizerEncode = NULL;
static fn_OgaCreateGeneratorParams p_OgaCreateGeneratorParams = NULL;
static fn_OgaDestroyGeneratorParams p_OgaDestroyGeneratorParams = NULL;
static fn_OgaGeneratorParamsSetSearchNumber p_OgaGeneratorParamsSetSearchNumber = NULL;
static fn_OgaGeneratorParamsSetSearchBool p_OgaGeneratorParamsSetSearchBool = NULL;
static fn_OgaCreateGenerator p_OgaCreateGenerator = NULL;
static fn_OgaDestroyGenerator p_OgaDestroyGenerator = NULL;
static fn_OgaGenerator_AppendTokenSequences p_OgaGenerator_AppendTokenSequences = NULL;
static fn_OgaGenerator_IsDone p_OgaGenerator_IsDone = NULL;
static fn_OgaGenerator_GenerateNextToken p_OgaGenerator_GenerateNextToken = NULL;
static fn_OgaGenerator_GetNextTokens p_OgaGenerator_GetNextTokens = NULL;
static fn_OgaResultGetError p_OgaResultGetError = NULL;
static fn_OgaDestroyResult p_OgaDestroyResult = NULL;

#define BIND_SYM(name) do { \
    p_##name = (fn_##name)dlsym(lib_handle, #name); \
    if (!p_##name) { \
        snprintf(last_error, sizeof(last_error), "Failed to find symbol %s: %s", #name, dlerror()); \
        return -2; \
    } \
} while(0)

int InitGenAI(const char* lib_path) {
    if (lib_handle) return 0;
    if (!lib_path || !lib_path[0]) lib_path = "libonnxruntime-genai.so";
    lib_handle = dlopen(lib_path, RTLD_NOW | RTLD_GLOBAL);
    if (!lib_handle) {
        snprintf(last_error, sizeof(last_error), "dlopen(%s) failed: %s", lib_path, dlerror());
        return -1;
    }

    BIND_SYM(OgaCreateModel);
    BIND_SYM(OgaDestroyModel);
    BIND_SYM(OgaCreateTokenizer);
    BIND_SYM(OgaDestroyTokenizer);
    BIND_SYM(OgaCreateTokenizerStream);
    BIND_SYM(OgaDestroyTokenizerStream);
    BIND_SYM(OgaTokenizerStreamDecode);
    BIND_SYM(OgaCreateSequences);
    BIND_SYM(OgaDestroySequences);
    BIND_SYM(OgaSequencesGetSequenceCount);
    BIND_SYM(OgaSequencesGetSequenceData);
    BIND_SYM(OgaTokenizerEncode);
    BIND_SYM(OgaCreateGeneratorParams);
    BIND_SYM(OgaDestroyGeneratorParams);
    BIND_SYM(OgaGeneratorParamsSetSearchNumber);
    BIND_SYM(OgaGeneratorParamsSetSearchBool);
    BIND_SYM(OgaCreateGenerator);
    BIND_SYM(OgaDestroyGenerator);
    BIND_SYM(OgaGenerator_AppendTokenSequences);
    BIND_SYM(OgaGenerator_IsDone);
    BIND_SYM(OgaGenerator_GenerateNextToken);
    BIND_SYM(OgaGenerator_GetNextTokens);
    BIND_SYM(OgaResultGetError);
    BIND_SYM(OgaDestroyResult);

    return 0;
}

const char* GetGenAILastError() {
    return last_error;
}

OgaResult* GenAI_CreateModel(const char* model_path, OgaModel** out) {
    if (!p_OgaCreateModel) return NULL;
    return p_OgaCreateModel(model_path, out);
}
void GenAI_DestroyModel(OgaModel* model) {
    if (p_OgaDestroyModel && model) p_OgaDestroyModel(model);
}

OgaResult* GenAI_CreateTokenizer(const OgaModel* model, OgaTokenizer** out) {
    if (!p_OgaCreateTokenizer) return NULL;
    return p_OgaCreateTokenizer(model, out);
}
void GenAI_DestroyTokenizer(OgaTokenizer* tokenizer) {
    if (p_OgaDestroyTokenizer && tokenizer) p_OgaDestroyTokenizer(tokenizer);
}

OgaResult* GenAI_CreateTokenizerStream(const OgaTokenizer* tokenizer, OgaTokenizerStream** out) {
    if (!p_OgaCreateTokenizerStream) return NULL;
    return p_OgaCreateTokenizerStream(tokenizer, out);
}
void GenAI_DestroyTokenizerStream(OgaTokenizerStream* stream) {
    if (p_OgaDestroyTokenizerStream && stream) p_OgaDestroyTokenizerStream(stream);
}
OgaResult* GenAI_TokenizerStreamDecode(OgaTokenizerStream* stream, int32_t token, const char** out) {
    if (!p_OgaTokenizerStreamDecode) return NULL;
    return p_OgaTokenizerStreamDecode(stream, token, out);
}

OgaResult* GenAI_CreateSequences(OgaSequences** out) {
    if (!p_OgaCreateSequences) return NULL;
    return p_OgaCreateSequences(out);
}
void GenAI_DestroySequences(OgaSequences* sequences) {
    if (p_OgaDestroySequences && sequences) p_OgaDestroySequences(sequences);
}
size_t GenAI_SequencesGetSequenceCount(const OgaSequences* sequences, size_t sequence_index) {
    if (!p_OgaSequencesGetSequenceCount || !sequences) return 0;
    return p_OgaSequencesGetSequenceCount(sequences, sequence_index);
}
const int32_t* GenAI_SequencesGetSequenceData(const OgaSequences* sequences, size_t sequence_index) {
    if (!p_OgaSequencesGetSequenceData || !sequences) return NULL;
    return p_OgaSequencesGetSequenceData(sequences, sequence_index);
}
OgaResult* GenAI_TokenizerEncode(const OgaTokenizer* tokenizer, const char* str, OgaSequences* sequences) {
    if (!p_OgaTokenizerEncode) return NULL;
    return p_OgaTokenizerEncode(tokenizer, str, sequences);
}

OgaResult* GenAI_CreateGeneratorParams(const OgaModel* model, OgaGeneratorParams** out) {
    if (!p_OgaCreateGeneratorParams) return NULL;
    return p_OgaCreateGeneratorParams(model, out);
}
void GenAI_DestroyGeneratorParams(OgaGeneratorParams* params) {
    if (p_OgaDestroyGeneratorParams && params) p_OgaDestroyGeneratorParams(params);
}
OgaResult* GenAI_GeneratorParamsSetSearchNumber(OgaGeneratorParams* params, const char* name, double value) {
    if (!p_OgaGeneratorParamsSetSearchNumber) return NULL;
    return p_OgaGeneratorParamsSetSearchNumber(params, name, value);
}
OgaResult* GenAI_GeneratorParamsSetSearchBool(OgaGeneratorParams* params, const char* name, bool value) {
    if (!p_OgaGeneratorParamsSetSearchBool) return NULL;
    return p_OgaGeneratorParamsSetSearchBool(params, name, value);
}

OgaResult* GenAI_CreateGenerator(const OgaModel* model, const OgaGeneratorParams* params, OgaGenerator** out) {
    if (!p_OgaCreateGenerator) return NULL;
    return p_OgaCreateGenerator(model, params, out);
}
void GenAI_DestroyGenerator(OgaGenerator* generator) {
    if (p_OgaDestroyGenerator && generator) p_OgaDestroyGenerator(generator);
}
OgaResult* GenAI_Generator_AppendTokenSequences(OgaGenerator* generator, const OgaSequences* sequences) {
    if (!p_OgaGenerator_AppendTokenSequences) return NULL;
    return p_OgaGenerator_AppendTokenSequences(generator, sequences);
}
bool GenAI_Generator_IsDone(OgaGenerator* generator) {
    if (!p_OgaGenerator_IsDone) return true;
    return p_OgaGenerator_IsDone(generator);
}
OgaResult* GenAI_Generator_GenerateNextToken(OgaGenerator* generator) {
    if (!p_OgaGenerator_GenerateNextToken) return NULL;
    return p_OgaGenerator_GenerateNextToken(generator);
}
OgaResult* GenAI_Generator_GetNextTokens(const OgaGenerator* generator, const int32_t** out, size_t* out_count) {
    if (!p_OgaGenerator_GetNextTokens) return NULL;
    return p_OgaGenerator_GetNextTokens(generator, out, out_count);
}

const char* GenAI_ResultGetError(const OgaResult* result) {
    if (!p_OgaResultGetError || !result) return "";
    return p_OgaResultGetError(result);
}
void GenAI_DestroyResult(OgaResult* result) {
    if (p_OgaDestroyResult && result) p_OgaDestroyResult(result);
}

