"""
AI Services Authentication and Authorization Middleware for Python
Supports FastAPI, Flask, and Django frameworks
"""

import json
import time
from typing import Dict, List, Optional, Any, Callable
from dataclasses import dataclass
from functools import wraps
import requests
import jwt
from jwt import PyJWTError


@dataclass
class AuthConfig:
    """Configuration for authentication middleware"""
    authorization_service_url: str
    jwt_public_key_url: str
    jwt_issuer: str
    jwt_audience: str
    service_name: str
    timeout: int = 10


@dataclass
class UserContext:
    """User context extracted from JWT token"""
    user_id: str
    username: str
    email: str
    roles: List[str]


@dataclass
class AuthorizationRequest:
    """Request to authorization service"""
    user_id: str
    resource: str
    action: str
    context: Optional[Dict[str, Any]] = None


@dataclass
class AuthorizationResponse:
    """Response from authorization service"""
    allowed: bool
    reason: Optional[str] = None


class AuthMiddleware:
    """Authentication and Authorization middleware for AI services"""
    
    def __init__(self, config: AuthConfig):
        self.config = config
        self.session = requests.Session()
        self.session.timeout = config.timeout
        
    def validate_jwt(self, token: str) -> UserContext:
        """Validate JWT token and extract user context"""
        # Remove Bearer prefix if present
        token = token.replace("Bearer ", "")
        
        try:
            # In production, fetch and verify with Keycloak's public key
            # For now, we'll decode without verification for demo
            decoded = jwt.decode(token, options={"verify_signature": False})
            
            # Extract user information
            user_context = UserContext(
                user_id=decoded.get("sub", ""),
                username=decoded.get("preferred_username", ""),
                email=decoded.get("email", ""),
                roles=decoded.get("realm_access", {}).get("roles", [])
            )
            
            return user_context
            
        except PyJWTError as e:
            raise ValueError(f"Invalid JWT token: {e}")
    
    def check_permission(self, user_id: str, resource: str, action: str, 
                        context: Optional[Dict[str, Any]] = None) -> AuthorizationResponse:
        """Check if user has permission for resource and action"""
        
        request_data = {
            "user_id": user_id,
            "resource": resource,
            "action": action,
            "context": context or {}
        }
        
        headers = {
            "Content-Type": "application/json",
            "X-Service-Name": self.config.service_name
        }
        
        try:
            response = self.session.post(
                f"{self.config.authorization_service_url}/v1/auth/check",
                json=request_data,
                headers=headers
            )
            
            if response.status_code == 200:
                data = response.json()
                return AuthorizationResponse(
                    allowed=data.get("allowed", False),
                    reason=data.get("reason")
                )
            else:
                return AuthorizationResponse(
                    allowed=False,
                    reason=f"Authorization service error: {response.status_code}"
                )
                
        except requests.RequestException as e:
            return AuthorizationResponse(
                allowed=False,
                reason=f"Authorization service unavailable: {e}"
            )
    
    def require_auth(self, resource: str, action: str):
        """Decorator for protecting endpoints with authentication and authorization"""
        def decorator(func: Callable) -> Callable:
            @wraps(func)
            def wrapper(*args, **kwargs):
                # This is a generic wrapper - specific implementations below
                return func(*args, **kwargs)
            return wrapper
        return decorator


class FastAPIAuthMiddleware(AuthMiddleware):
    """FastAPI-specific authentication middleware"""
    
    def require_auth(self, resource: str, action: str):
        """FastAPI decorator for authentication and authorization"""
        def decorator(func: Callable) -> Callable:
            @wraps(func)
            async def wrapper(*args, **kwargs):
                from fastapi import HTTPException, Request
                
                # Extract request object
                request = None
                for arg in args:
                    if isinstance(arg, Request):
                        request = arg
                        break
                
                if not request:
                    raise HTTPException(status_code=500, detail="Request object not found")
                
                # Extract JWT from Authorization header
                auth_header = request.headers.get("Authorization")
                if not auth_header:
                    raise HTTPException(status_code=401, detail="Authorization header required")
                
                try:
                    # Validate JWT and extract user context
                    user_context = self.validate_jwt(auth_header)
                    
                    # Check permission
                    auth_response = self.check_permission(
                        user_context.user_id, resource, action
                    )
                    
                    if not auth_response.allowed:
                        raise HTTPException(
                            status_code=403, 
                            detail=f"Access denied: {auth_response.reason}"
                        )
                    
                    # Add user context to request state
                    request.state.user = user_context
                    
                    return await func(*args, **kwargs)
                    
                except ValueError as e:
                    raise HTTPException(status_code=401, detail=str(e))
                except Exception as e:
                    raise HTTPException(status_code=500, detail=f"Authentication error: {e}")
            
            return wrapper
        return decorator


class FlaskAuthMiddleware(AuthMiddleware):
    """Flask-specific authentication middleware"""
    
    def require_auth(self, resource: str, action: str):
        """Flask decorator for authentication and authorization"""
        def decorator(func: Callable) -> Callable:
            @wraps(func)
            def wrapper(*args, **kwargs):
                from flask import request, jsonify, g
                
                # Extract JWT from Authorization header
                auth_header = request.headers.get("Authorization")
                if not auth_header:
                    return jsonify({"error": "Authorization header required"}), 401
                
                try:
                    # Validate JWT and extract user context
                    user_context = self.validate_jwt(auth_header)
                    
                    # Check permission
                    auth_response = self.check_permission(
                        user_context.user_id, resource, action
                    )
                    
                    if not auth_response.allowed:
                        return jsonify({
                            "error": f"Access denied: {auth_response.reason}"
                        }), 403
                    
                    # Add user context to Flask g object
                    g.user = user_context
                    
                    return func(*args, **kwargs)
                    
                except ValueError as e:
                    return jsonify({"error": str(e)}), 401
                except Exception as e:
                    return jsonify({"error": f"Authentication error: {e}"}), 500
            
            return wrapper
        return decorator


# Example usage for different frameworks

def create_fastapi_example():
    """Example FastAPI service with authentication"""
    from fastapi import FastAPI, Request
    
    app = FastAPI(title="AI Model Management Service")
    
    # Initialize auth middleware
    auth_config = AuthConfig(
        authorization_service_url="http://localhost:8081",
        jwt_public_key_url="http://localhost:8080/realms/002aic/protocol/openid-connect/certs",
        jwt_issuer="http://localhost:8080/realms/002aic",
        jwt_audience="002aic-api",
        service_name="model-management-service"
    )
    
    auth = FastAPIAuthMiddleware(auth_config)
    
    @app.get("/models")
    @auth.require_auth("model", "read")
    async def list_models(request: Request):
        user = request.state.user
        return {
            "models": ["model1", "model2"],
            "user": user.username
        }
    
    @app.post("/models")
    @auth.require_auth("model", "create")
    async def create_model(request: Request):
        user = request.state.user
        return {
            "message": "Model created",
            "created_by": user.username
        }
    
    return app


def create_flask_example():
    """Example Flask service with authentication"""
    from flask import Flask, g
    
    app = Flask(__name__)
    
    # Initialize auth middleware
    auth_config = AuthConfig(
        authorization_service_url="http://localhost:8081",
        jwt_public_key_url="http://localhost:8080/realms/002aic/protocol/openid-connect/certs",
        jwt_issuer="http://localhost:8080/realms/002aic",
        jwt_audience="002aic-api",
        service_name="analytics-service"
    )
    
    auth = FlaskAuthMiddleware(auth_config)
    
    @app.route("/analytics", methods=["GET"])
    @auth.require_auth("analytics", "read")
    def get_analytics():
        return {
            "analytics": {"users": 100, "models": 50},
            "user": g.user.username
        }
    
    @app.route("/analytics/reports", methods=["POST"])
    @auth.require_auth("analytics", "create")
    def create_report():
        return {
            "message": "Report created",
            "created_by": g.user.username
        }
    
    return app


# Utility functions for service integration

def get_auth_headers(user_context: UserContext) -> Dict[str, str]:
    """Generate headers for service-to-service communication"""
    return {
        "X-User-ID": user_context.user_id,
        "X-User-Email": user_context.email,
        "X-User-Roles": ",".join(user_context.roles),
        "X-Service-Auth": "internal"
    }


def create_service_client(service_url: str, user_context: UserContext) -> requests.Session:
    """Create HTTP client for calling other AI services"""
    session = requests.Session()
    session.headers.update(get_auth_headers(user_context))
    session.base_url = service_url
    return session
