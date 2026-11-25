#pragma once
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct OgaResult OgaResult;
typedef struct OgaModel OgaModel;
typedef struct OgaTokenizer OgaTokenizer;
typedef struct OgaTokenizerStream OgaTokenizerStream;
typedef struct OgaSequences OgaSequences;
typedef struct OgaGeneratorParams OgaGeneratorParams;
typedef struct OgaGenerator OgaGenerator;

int InitGenAI(const char* lib_path);
const char* GetGenAILastError();

OgaResult* GenAI_CreateModel(const char* model_path, OgaModel** out);
void GenAI_DestroyModel(OgaModel* model);

OgaResult* GenAI_CreateTokenizer(const OgaModel* model, OgaTokenizer** out);
void GenAI_DestroyTokenizer(OgaTokenizer* tokenizer);

OgaResult* GenAI_CreateTokenizerStream(const OgaTokenizer* tokenizer, OgaTokenizerStream** out);
void GenAI_DestroyTokenizerStream(OgaTokenizerStream* stream);
OgaResult* GenAI_TokenizerStreamDecode(OgaTokenizerStream* stream, int32_t token, const char** out);

OgaResult* GenAI_CreateSequences(OgaSequences** out);
void GenAI_DestroySequences(OgaSequences* sequences);
size_t GenAI_SequencesGetSequenceCount(const OgaSequences* sequences, size_t sequence_index);
const int32_t* GenAI_SequencesGetSequenceData(const OgaSequences* sequences, size_t sequence_index);
OgaResult* GenAI_TokenizerEncode(const OgaTokenizer* tokenizer, const char* str, OgaSequences* sequences);

OgaResult* GenAI_CreateGeneratorParams(const OgaModel* model, OgaGeneratorParams** out);
void GenAI_DestroyGeneratorParams(OgaGeneratorParams* params);
OgaResult* GenAI_GeneratorParamsSetSearchNumber(OgaGeneratorParams* params, const char* name, double value);
OgaResult* GenAI_GeneratorParamsSetSearchBool(OgaGeneratorParams* params, const char* name, bool value);

OgaResult* GenAI_CreateGenerator(const OgaModel* model, const OgaGeneratorParams* params, OgaGenerator** out);
void GenAI_DestroyGenerator(OgaGenerator* generator);
OgaResult* GenAI_Generator_AppendTokenSequences(OgaGenerator* generator, const OgaSequences* sequences);
bool GenAI_Generator_IsDone(OgaGenerator* generator);
OgaResult* GenAI_Generator_GenerateNextToken(OgaGenerator* generator);
OgaResult* GenAI_Generator_GetNextTokens(const OgaGenerator* generator, const int32_t** out, size_t* out_count);

const char* GenAI_ResultGetError(const OgaResult* result);
void GenAI_DestroyResult(OgaResult* result);

#ifdef __cplusplus
}
#endif

