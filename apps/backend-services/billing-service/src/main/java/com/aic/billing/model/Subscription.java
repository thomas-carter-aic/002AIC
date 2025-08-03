package com.aic.billing.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.ToString;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.UpdateTimestamp;

import javax.persistence.*;
import javax.validation.constraints.NotBlank;
import javax.validation.constraints.NotNull;
import javax.validation.constraints.Positive;
import javax.validation.constraints.Size;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "subscriptions")
@Data
@EqualsAndHashCode(exclude = {"customer", "subscriptionItems", "invoices"})
@ToString(exclude = {"customer", "subscriptionItems", "invoices"})
public class Subscription {

    @Id
    @Column(name = "id", length = 36)
    private String id;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "customer_id", nullable = false)
    @JsonIgnore
    private Customer customer;

    @NotBlank
    @Size(max = 255)
    @Column(name = "plan_id", nullable = false)
    private String planId;

    @NotBlank
    @Size(max = 255)
    @Column(name = "plan_name", nullable = false)
    private String planName;

    @NotNull
    @Positive
    @Column(name = "quantity", nullable = false)
    private Integer quantity = 1;

    @NotNull
    @Column(name = "unit_price", precision = 12, scale = 2, nullable = false)
    private BigDecimal unitPrice;

    @Column(name = "discount_amount", precision = 12, scale = 2)
    private BigDecimal discountAmount = BigDecimal.ZERO;

    @Column(name = "discount_percent", precision = 5, scale = 2)
    private BigDecimal discountPercent = BigDecimal.ZERO;

    @Size(max = 50)
    @Column(name = "billing_cycle")
    @Enumerated(EnumType.STRING)
    private BillingCycle billingCycle = BillingCycle.MONTHLY;

    @Size(max = 50)
    @Column(name = "status")
    @Enumerated(EnumType.STRING)
    private SubscriptionStatus status = SubscriptionStatus.ACTIVE;

    @Column(name = "trial_start")
    private LocalDateTime trialStart;

    @Column(name = "trial_end")
    private LocalDateTime trialEnd;

    @Column(name = "current_period_start", nullable = false)
    private LocalDateTime currentPeriodStart;

    @Column(name = "current_period_end", nullable = false)
    private LocalDateTime currentPeriodEnd;

    @Column(name = "cancel_at_period_end")
    private Boolean cancelAtPeriodEnd = false;

    @Column(name = "cancelled_at")
    private LocalDateTime cancelledAt;

    @Column(name = "ended_at")
    private LocalDateTime endedAt;

    @Size(max = 255)
    @Column(name = "stripe_subscription_id")
    private String stripeSubscriptionId;

    @Column(name = "metadata", columnDefinition = "TEXT")
    private String metadata;

    @CreationTimestamp
    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @OneToMany(mappedBy = "subscription", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<SubscriptionItem> subscriptionItems = new ArrayList<>();

    @OneToMany(mappedBy = "subscription", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<Invoice> invoices = new ArrayList<>();

    public enum BillingCycle {
        DAILY, WEEKLY, MONTHLY, QUARTERLY, YEARLY
    }

    public enum SubscriptionStatus {
        ACTIVE, TRIALING, PAST_DUE, CANCELLED, UNPAID, INCOMPLETE, INCOMPLETE_EXPIRED
    }

    // Helper methods
    public BigDecimal calculateTotalAmount() {
        BigDecimal total = unitPrice.multiply(BigDecimal.valueOf(quantity));
        
        if (discountAmount != null && discountAmount.compareTo(BigDecimal.ZERO) > 0) {
            total = total.subtract(discountAmount);
        }
        
        if (discountPercent != null && discountPercent.compareTo(BigDecimal.ZERO) > 0) {
            BigDecimal discount = total.multiply(discountPercent).divide(BigDecimal.valueOf(100));
            total = total.subtract(discount);
        }
        
        return total.max(BigDecimal.ZERO);
    }

    public boolean isInTrial() {
        LocalDateTime now = LocalDateTime.now();
        return trialStart != null && trialEnd != null && 
               now.isAfter(trialStart) && now.isBefore(trialEnd);
    }

    public boolean isActive() {
        return status == SubscriptionStatus.ACTIVE || status == SubscriptionStatus.TRIALING;
    }
}
