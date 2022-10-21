// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

#include "textflag.h"

// these stubs are here because it is not possible to go:linkname directly the C functions on darwin arm64

TEXT _malloc(SB), NOSPLIT, $0-0
	JMP purego_malloc(SB)
	RET

TEXT _free(SB), NOSPLIT, $0-0
	JMP purego_free(SB)
	RET

TEXT _setenv(SB), NOSPLIT, $0-0
	JMP purego_setenv(SB)
	RET

TEXT _unsetenv(SB), NOSPLIT, $0-0
	JMP purego_unsetenv(SB)
	RET

TEXT _pthread_attr_init(SB), NOSPLIT, $0-0
	JMP purego_pthread_attr_init(SB)
	RET

TEXT _pthread_create(SB), NOSPLIT, $0-0
	JMP purego_pthread_create(SB)
	RET

TEXT _pthread_detach(SB), NOSPLIT, $0-12
	JMP purego_pthread_detach(SB)
	RET

TEXT _pthread_sigmask(SB), NOSPLIT, $0-0
	JMP purego_pthread_sigmask(SB)
	RET

TEXT _pthread_attr_getstacksize(SB), NOSPLIT, $0-0
	JMP purego_pthread_attr_getstacksize(SB)
	RET

TEXT _pthread_attr_destroy(SB), NOSPLIT, $0-0
	JMP purego_pthread_attr_destroy(SB)
	RET

TEXT _abort(SB), NOSPLIT, $0-0
	JMP purego_abort(SB)
	RET

TEXT _nanosleep(SB), NOSPLIT, $0-0
	JMP purego_nanosleep(SB)
	RET

TEXT _sigfillset(SB), NOSPLIT, $0-12
	CALL purego_sigfillset(SB)
	RET
