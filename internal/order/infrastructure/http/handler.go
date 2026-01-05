package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dmehra2102/Order-Management-System/internal/order/application"
	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	log     *slog.Logger
	service *application.Service
	tracer  trace.Tracer
}

func NewHandler(log *slog.Logger, service *application.Service) *Handler {
	return &Handler{
		log:     log,
		service: service,
		tracer:  otel.Tracer("order-http"),
	}
}

type createOrderReq struct {
	ID       string             `json:"id"`
	Customer string             `json:"customer"`
	Items    []domain.OrderItem `json:"items"`
	Headers  map[string]string  `json:"headers"`
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/orders", h.createOrder)
	r.Get("/orders/{id}", h.getOrder)

	return r
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "CreateOrder")
	defer span.End()

	var req createOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	o := domain.NewOrder(req.ID, req.Customer, req.Items)

	traceparent := r.Header.Get("traceparent")
	if traceparent == "" {
		carrier := map[string]string{}
		otel.GetTextMapPropagator().Inject(ctx, propagationMapCarrier(carrier))
		traceparent = carrier["traceparent"]
	}

	if err := h.service.CreateOrder(ctx, o, req.Headers, traceparent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending", "order_id": o.ID})
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"message":"not implemented this part as of now"}`))
}

type propagationMapCarrier map[string]string

func (c propagationMapCarrier) Get(key string) string { return c[key] }
func (c propagationMapCarrier) Set(key, val string)   { c[key] = val }
func (c propagationMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
