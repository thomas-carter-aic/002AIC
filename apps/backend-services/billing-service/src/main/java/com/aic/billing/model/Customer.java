package com.aic.billing.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.ToString;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.UpdateTimestamp;

import javax.persistence.*;
import javax.validation.constraints.Email;
import javax.validation.constraints.NotBlank;
import javax.validation.constraints.Size;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "customers")
@Data
@EqualsAndHashCode(exclude = {"subscriptions", "paymentMethods", "invoices"})
@ToString(exclude = {"subscriptions", "paymentMethods", "invoices"})
public class Customer {

    @Id
    @Column(name = "id", length = 36)
    private String id;

    @NotBlank
    @Size(max = 255)
    @Column(name = "user_id", nullable = false, unique = true)
    private String userId;

    @NotBlank
    @Size(max = 255)
    @Column(name = "email", nullable = false)
    @Email
    private String email;

    @Size(max = 255)
    @Column(name = "company_name")
    private String companyName;

    @Size(max = 100)
    @Column(name = "first_name")
    private String firstName;

    @Size(max = 100)
    @Column(name = "last_name")
    private String lastName;

    @Size(max = 20)
    @Column(name = "phone")
    private String phone;

    @Embedded
    private Address billingAddress;

    @Size(max = 100)
    @Column(name = "tax_id")
    private String taxId;

    @Column(name = "tax_rate", precision = 5, scale = 4)
    private BigDecimal taxRate = BigDecimal.ZERO;

    @Size(max = 3)
    @Column(name = "currency", length = 3)
    private String currency = "USD";

    @Size(max = 50)
    @Column(name = "status")
    @Enumerated(EnumType.STRING)
    private CustomerStatus status = CustomerStatus.ACTIVE;

    @Column(name = "account_balance", precision = 12, scale = 2)
    private BigDecimal accountBalance = BigDecimal.ZERO;

    @Column(name = "credit_limit", precision = 12, scale = 2)
    private BigDecimal creditLimit = BigDecimal.ZERO;

    @Size(max = 255)
    @Column(name = "stripe_customer_id")
    private String stripeCustomerId;

    @Column(name = "metadata", columnDefinition = "TEXT")
    private String metadata;

    @CreationTimestamp
    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @OneToMany(mappedBy = "customer", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<Subscription> subscriptions = new ArrayList<>();

    @OneToMany(mappedBy = "customer", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<PaymentMethod> paymentMethods = new ArrayList<>();

    @OneToMany(mappedBy = "customer", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<Invoice> invoices = new ArrayList<>();

    public enum CustomerStatus {
        ACTIVE, SUSPENDED, DELINQUENT, CANCELLED
    }

    @Embeddable
    @Data
    public static class Address {
        @Size(max = 255)
        @Column(name = "address_line1")
        private String line1;

        @Size(max = 255)
        @Column(name = "address_line2")
        private String line2;

        @Size(max = 100)
        @Column(name = "address_city")
        private String city;

        @Size(max = 100)
        @Column(name = "address_state")
        private String state;

        @Size(max = 20)
        @Column(name = "address_postal_code")
        private String postalCode;

        @Size(max = 2)
        @Column(name = "address_country", length = 2)
        private String country;
    }
}
