package com.aic.billing.controller;

import com.aic.billing.dto.*;
import com.aic.billing.model.Customer;
import com.aic.billing.model.Invoice;
import com.aic.billing.model.PaymentMethod;
import com.aic.billing.model.Subscription;
import com.aic.billing.service.BillingService;
import com.aic.billing.service.CustomerService;
import com.aic.billing.service.InvoiceService;
import com.aic.billing.service.SubscriptionService;
import io.micrometer.core.annotation.Timed;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.tags.Tag;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.*;

import javax.validation.Valid;
import javax.validation.constraints.NotBlank;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/v1")
@RequiredArgsConstructor
@Validated
@Slf4j
@Tag(name = "Billing", description = "Billing and subscription management API")
public class BillingController {

    private final BillingService billingService;
    private final CustomerService customerService;
    private final SubscriptionService subscriptionService;
    private final InvoiceService invoiceService;

    // Health check
    @GetMapping("/health")
    @Operation(summary = "Health check", description = "Check service health status")
    public ResponseEntity<Map<String, Object>> healthCheck() {
        return ResponseEntity.ok(billingService.getHealthStatus());
    }

    // Customer Management
    @PostMapping("/customers")
    @Operation(summary = "Create customer", description = "Create a new billing customer")
    @Timed(value = "billing.customer.create", description = "Time taken to create customer")
    public ResponseEntity<CustomerResponse> createCustomer(
            @Valid @RequestBody CreateCustomerRequest request) {
        log.info("Creating customer for user: {}", request.getUserId());
        Customer customer = customerService.createCustomer(request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(CustomerResponse.fromEntity(customer));
    }

    @GetMapping("/customers/{customerId}")
    @Operation(summary = "Get customer", description = "Retrieve customer by ID")
    public ResponseEntity<CustomerResponse> getCustomer(
            @Parameter(description = "Customer ID") @PathVariable @NotBlank String customerId) {
        Customer customer = customerService.getCustomer(customerId);
        return ResponseEntity.ok(CustomerResponse.fromEntity(customer));
    }

    @GetMapping("/customers")
    @Operation(summary = "List customers", description = "List all customers with pagination")
    public ResponseEntity<Page<CustomerResponse>> listCustomers(
            @Parameter(description = "User ID filter") @RequestParam(required = false) String userId,
            @Parameter(description = "Status filter") @RequestParam(required = false) String status,
            Pageable pageable) {
        Page<Customer> customers = customerService.listCustomers(userId, status, pageable);
        Page<CustomerResponse> response = customers.map(CustomerResponse::fromEntity);
        return ResponseEntity.ok(response);
    }

    @PutMapping("/customers/{customerId}")
    @Operation(summary = "Update customer", description = "Update customer information")
    @Timed(value = "billing.customer.update", description = "Time taken to update customer")
    public ResponseEntity<CustomerResponse> updateCustomer(
            @Parameter(description = "Customer ID") @PathVariable @NotBlank String customerId,
            @Valid @RequestBody UpdateCustomerRequest request) {
        log.info("Updating customer: {}", customerId);
        Customer customer = customerService.updateCustomer(customerId, request);
        return ResponseEntity.ok(CustomerResponse.fromEntity(customer));
    }

    @DeleteMapping("/customers/{customerId}")
    @Operation(summary = "Delete customer", description = "Delete a customer")
    @Timed(value = "billing.customer.delete", description = "Time taken to delete customer")
    public ResponseEntity<Void> deleteCustomer(
            @Parameter(description = "Customer ID") @PathVariable @NotBlank String customerId) {
        log.info("Deleting customer: {}", customerId);
        customerService.deleteCustomer(customerId);
        return ResponseEntity.noContent().build();
    }

    // Payment Methods
    @PostMapping("/customers/{customerId}/payment-methods")
    @Operation(summary = "Add payment method", description = "Add a payment method to customer")
    @Timed(value = "billing.payment_method.create", description = "Time taken to create payment method")
    public ResponseEntity<PaymentMethodResponse> addPaymentMethod(
            @Parameter(description = "Customer ID") @PathVariable @NotBlank String customerId,
            @Valid @RequestBody CreatePaymentMethodRequest request) {
        log.info("Adding payment method for customer: {}", customerId);
        PaymentMethod paymentMethod = billingService.addPaymentMethod(customerId, request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(PaymentMethodResponse.fromEntity(paymentMethod));
    }

    @GetMapping("/customers/{customerId}/payment-methods")
    @Operation(summary = "List payment methods", description = "List customer payment methods")
    public ResponseEntity<List<PaymentMethodResponse>> listPaymentMethods(
            @Parameter(description = "Customer ID") @PathVariable @NotBlank String customerId) {
        List<PaymentMethod> paymentMethods = billingService.listPaymentMethods(customerId);
        List<PaymentMethodResponse> response = paymentMethods.stream()
                .map(PaymentMethodResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }

    @DeleteMapping("/payment-methods/{paymentMethodId}")
    @Operation(summary = "Delete payment method", description = "Delete a payment method")
    @Timed(value = "billing.payment_method.delete", description = "Time taken to delete payment method")
    public ResponseEntity<Void> deletePaymentMethod(
            @Parameter(description = "Payment Method ID") @PathVariable @NotBlank String paymentMethodId) {
        log.info("Deleting payment method: {}", paymentMethodId);
        billingService.deletePaymentMethod(paymentMethodId);
        return ResponseEntity.noContent().build();
    }

    // Subscription Management
    @PostMapping("/subscriptions")
    @Operation(summary = "Create subscription", description = "Create a new subscription")
    @Timed(value = "billing.subscription.create", description = "Time taken to create subscription")
    public ResponseEntity<SubscriptionResponse> createSubscription(
            @Valid @RequestBody CreateSubscriptionRequest request) {
        log.info("Creating subscription for customer: {}", request.getCustomerId());
        Subscription subscription = subscriptionService.createSubscription(request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(SubscriptionResponse.fromEntity(subscription));
    }

    @GetMapping("/subscriptions/{subscriptionId}")
    @Operation(summary = "Get subscription", description = "Retrieve subscription by ID")
    public ResponseEntity<SubscriptionResponse> getSubscription(
            @Parameter(description = "Subscription ID") @PathVariable @NotBlank String subscriptionId) {
        Subscription subscription = subscriptionService.getSubscription(subscriptionId);
        return ResponseEntity.ok(SubscriptionResponse.fromEntity(subscription));
    }

    @GetMapping("/subscriptions")
    @Operation(summary = "List subscriptions", description = "List subscriptions with pagination")
    public ResponseEntity<Page<SubscriptionResponse>> listSubscriptions(
            @Parameter(description = "Customer ID filter") @RequestParam(required = false) String customerId,
            @Parameter(description = "Status filter") @RequestParam(required = false) String status,
            @Parameter(description = "Plan ID filter") @RequestParam(required = false) String planId,
            Pageable pageable) {
        Page<Subscription> subscriptions = subscriptionService.listSubscriptions(
                customerId, status, planId, pageable);
        Page<SubscriptionResponse> response = subscriptions.map(SubscriptionResponse::fromEntity);
        return ResponseEntity.ok(response);
    }

    @PutMapping("/subscriptions/{subscriptionId}")
    @Operation(summary = "Update subscription", description = "Update subscription details")
    @Timed(value = "billing.subscription.update", description = "Time taken to update subscription")
    public ResponseEntity<SubscriptionResponse> updateSubscription(
            @Parameter(description = "Subscription ID") @PathVariable @NotBlank String subscriptionId,
            @Valid @RequestBody UpdateSubscriptionRequest request) {
        log.info("Updating subscription: {}", subscriptionId);
        Subscription subscription = subscriptionService.updateSubscription(subscriptionId, request);
        return ResponseEntity.ok(SubscriptionResponse.fromEntity(subscription));
    }

    @PostMapping("/subscriptions/{subscriptionId}/cancel")
    @Operation(summary = "Cancel subscription", description = "Cancel a subscription")
    @Timed(value = "billing.subscription.cancel", description = "Time taken to cancel subscription")
    public ResponseEntity<SubscriptionResponse> cancelSubscription(
            @Parameter(description = "Subscription ID") @PathVariable @NotBlank String subscriptionId,
            @RequestParam(defaultValue = "true") boolean atPeriodEnd) {
        log.info("Cancelling subscription: {} at period end: {}", subscriptionId, atPeriodEnd);
        Subscription subscription = subscriptionService.cancelSubscription(subscriptionId, atPeriodEnd);
        return ResponseEntity.ok(SubscriptionResponse.fromEntity(subscription));
    }

    @PostMapping("/subscriptions/{subscriptionId}/reactivate")
    @Operation(summary = "Reactivate subscription", description = "Reactivate a cancelled subscription")
    @Timed(value = "billing.subscription.reactivate", description = "Time taken to reactivate subscription")
    public ResponseEntity<SubscriptionResponse> reactivateSubscription(
            @Parameter(description = "Subscription ID") @PathVariable @NotBlank String subscriptionId) {
        log.info("Reactivating subscription: {}", subscriptionId);
        Subscription subscription = subscriptionService.reactivateSubscription(subscriptionId);
        return ResponseEntity.ok(SubscriptionResponse.fromEntity(subscription));
    }

    // Invoice Management
    @GetMapping("/invoices/{invoiceId}")
    @Operation(summary = "Get invoice", description = "Retrieve invoice by ID")
    public ResponseEntity<InvoiceResponse> getInvoice(
            @Parameter(description = "Invoice ID") @PathVariable @NotBlank String invoiceId) {
        Invoice invoice = invoiceService.getInvoice(invoiceId);
        return ResponseEntity.ok(InvoiceResponse.fromEntity(invoice));
    }

    @GetMapping("/invoices")
    @Operation(summary = "List invoices", description = "List invoices with pagination")
    public ResponseEntity<Page<InvoiceResponse>> listInvoices(
            @Parameter(description = "Customer ID filter") @RequestParam(required = false) String customerId,
            @Parameter(description = "Status filter") @RequestParam(required = false) String status,
            @Parameter(description = "Subscription ID filter") @RequestParam(required = false) String subscriptionId,
            Pageable pageable) {
        Page<Invoice> invoices = invoiceService.listInvoices(customerId, status, subscriptionId, pageable);
        Page<InvoiceResponse> response = invoices.map(InvoiceResponse::fromEntity);
        return ResponseEntity.ok(response);
    }

    @PostMapping("/invoices/{invoiceId}/pay")
    @Operation(summary = "Pay invoice", description = "Process payment for an invoice")
    @Timed(value = "billing.invoice.pay", description = "Time taken to pay invoice")
    public ResponseEntity<PaymentResponse> payInvoice(
            @Parameter(description = "Invoice ID") @PathVariable @NotBlank String invoiceId,
            @Valid @RequestBody PayInvoiceRequest request) {
        log.info("Processing payment for invoice: {}", invoiceId);
        PaymentResponse payment = billingService.payInvoice(invoiceId, request);
        return ResponseEntity.ok(payment);
    }

    @PostMapping("/invoices/{invoiceId}/void")
    @Operation(summary = "Void invoice", description = "Void an invoice")
    @Timed(value = "billing.invoice.void", description = "Time taken to void invoice")
    public ResponseEntity<InvoiceResponse> voidInvoice(
            @Parameter(description = "Invoice ID") @PathVariable @NotBlank String invoiceId) {
        log.info("Voiding invoice: {}", invoiceId);
        Invoice invoice = invoiceService.voidInvoice(invoiceId);
        return ResponseEntity.ok(InvoiceResponse.fromEntity(invoice));
    }

    // Billing Analytics
    @GetMapping("/analytics/revenue")
    @Operation(summary = "Revenue analytics", description = "Get revenue analytics")
    public ResponseEntity<RevenueAnalyticsResponse> getRevenueAnalytics(
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate,
            @Parameter(description = "Group by (day, week, month)") @RequestParam(defaultValue = "month") String groupBy) {
        RevenueAnalyticsResponse analytics = billingService.getRevenueAnalytics(startDate, endDate, groupBy);
        return ResponseEntity.ok(analytics);
    }

    @GetMapping("/analytics/subscriptions")
    @Operation(summary = "Subscription analytics", description = "Get subscription analytics")
    public ResponseEntity<SubscriptionAnalyticsResponse> getSubscriptionAnalytics() {
        SubscriptionAnalyticsResponse analytics = billingService.getSubscriptionAnalytics();
        return ResponseEntity.ok(analytics);
    }

    @GetMapping("/analytics/customers")
    @Operation(summary = "Customer analytics", description = "Get customer analytics")
    public ResponseEntity<CustomerAnalyticsResponse> getCustomerAnalytics() {
        CustomerAnalyticsResponse analytics = billingService.getCustomerAnalytics();
        return ResponseEntity.ok(analytics);
    }

    // Webhook handling
    @PostMapping("/webhooks/stripe")
    @Operation(summary = "Stripe webhook", description = "Handle Stripe webhook events")
    public ResponseEntity<Void> handleStripeWebhook(
            @RequestBody String payload,
            @RequestHeader("Stripe-Signature") String signature) {
        log.info("Received Stripe webhook");
        billingService.handleStripeWebhook(payload, signature);
        return ResponseEntity.ok().build();
    }
}
