package nethttplibrary

import abs "github.com/microsoft/kiota-abstractions-go"

type HandlerType interface {
	// IsFinalHandler returns true if the current handler is the last one in the chain
	IsFinalHandler() bool
}

type ProcessHandler interface {
	HandlerType
	abs.RequestHandlerOption
	GetResponseHandler() abs.ResponseHandler
}

type processHandler struct {
	responseHandler abs.ResponseHandler
	isFinalHandler  bool
}

// NewProcessHandler creates a new ProcessHandler object
func NewProcessHandler(responseHandler abs.ResponseHandler, isFinalHandler bool) ProcessHandler {
	return &processHandler{
		responseHandler: responseHandler,
		isFinalHandler:  isFinalHandler,
	}
}

func (p *processHandler) GetResponseHandler() abs.ResponseHandler {
	return p.responseHandler
}

func (p *processHandler) IsFinalHandler() bool {
	return p.isFinalHandler && p.responseHandler != nil
}

func (p *processHandler) SetResponseHandler(responseHandler abs.ResponseHandler) {
	p.responseHandler = responseHandler
}

func (p *processHandler) GetKey() abs.RequestOptionKey {
	return abs.ResponseHandlerOptionKey
}
