/**
 * Marketplace Service - Java Spring Boot
 * Plugin and extension management for the 002AIC platform
 * Handles marketplace listings, plugin installations, version management, and revenue sharing
 */

package com.aic.marketplace;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.cloud.openfeign.EnableFeignClients;
import org.springframework.data.jpa.repository.config.EnableJpaRepositories;
import org.springframework.scheduling.annotation.EnableAsync;
import org.springframework.scheduling.annotation.EnableScheduling;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.springframework.cache.annotation.EnableCaching;

@SpringBootApplication
@EnableFeignClients
@EnableJpaRepositories
@EnableAsync
@EnableScheduling
@EnableTransactionManagement
@EnableCaching
public class MarketplaceServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(MarketplaceServiceApplication.class, args);
    }
}
