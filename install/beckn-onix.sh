#!/bin/bash
source scripts/variables.sh
source scripts/get_container_details.sh

# Function to start a specific service inside docker-compose file
install_package() {
    echo "${GREEN}................Installing required packages................${NC}"
    bash scripts/package_manager.sh
    echo "Package Installation is done"

}
start_container() {
    #ignore orphaned containers warning
    export COMPOSE_IGNORE_ORPHANS=1
    docker compose -f $1 up -d $2
}

update_registry_details() {
    if [[ $1 ]]; then
        if [[ $1 == https://* ]]; then
            if [[ $(uname -s) == 'Darwin' ]]; then
                registry_url=$(echo "$1" | sed -E 's/https:\/\///')
            else
                registry_url=$(echo "$1" | sed 's/https:\/\///')
            fi
            registry_port=443
            protocol=https
        elif [[ $1 == http://* ]]; then
            if [[ $(uname -s) == 'Darwin' ]]; then
                registry_url=$(echo "$1" | sed -E 's/http:\/\///')
            else
                registry_url=$(echo "$1" | sed 's/http:\/\///')
            fi
            registry_port=80
            protocol=http
        fi

    else
        registry_url=registry
        registry_port=3030
        protocol=http
    fi
    echo $registry_url
    cp $SCRIPT_DIR/../registry_data/config/swf.properties-sample $SCRIPT_DIR/../registry_data/config/swf.properties
    config_file="$SCRIPT_DIR/../registry_data/config/swf.properties"

    tmp_file=$(mktemp "tempfile.XXXXXXXXXX")
    sed "s|REGISTRY_URL|$registry_url|g; s|REGISTRY_PORT|$registry_port|g; s|PROTOCOL|$protocol|g" "$config_file" >"$tmp_file"
    mv "$tmp_file" "$config_file"
    docker volume create registry_data_volume
    docker volume create registry_database_volume
    docker run --rm -v $SCRIPT_DIR/../registry_data/config:/source -v registry_data_volume:/target busybox cp /source/{envvars,logger.properties,swf.properties} /target/
    docker rmi busybox
}
# Function to start Redis service only
start_support_services() {
    #ignore orphaned containers warning
    export COMPOSE_IGNORE_ORPHANS=1
    echo "${GREEN}................Installing Redis................${NC}"
    docker compose -f docker-compose-app.yml up -d redis_db
    echo "Redis installation successful"
}

install_gateway() {
    if [[ $1 && $2 ]]; then
        bash scripts/update_gateway_details.sh $1 $2
    else
        bash scripts/update_gateway_details.sh http://registry:3030
    fi
    echo "${GREEN}................Installing Gateway service................${NC}"
    start_container $gateway_docker_compose_file gateway
    echo "Registering Gateway in the registry"

    sleep 10
    # if [[ $1 && $2 ]]; then
    #     bash scripts/register_gateway.sh $2
    # else
    #     bash scripts/register_gateway.sh
    # fi
    echo " "
    echo "Gateway installation successful"
}

# Function to install Beckn Gateway and Beckn Registry
install_registry() {
    if [[ $1 ]]; then
        update_registry_details $1
    else
        update_registry_details
    fi

    echo "${GREEN}................Installing Registry service................${NC}"
    start_container $registry_docker_compose_file registry
    sleep 10
    echo "Registry installation successful"

    #Update Role Permission for registry.
    if [[ $1 ]]; then
        bash scripts/registry_role_permissions.sh $1
    else
        bash scripts/registry_role_permissions.sh
    fi
}

# Function to install Layer2 Config
install_layer2_config() {
    container_name=$1
    FILENAME="$(basename "$layer2_url")"
    wget -O "$(basename "$layer2_url")" "$layer2_url" >/dev/null 2>&1
    if [ $? -eq 0 ]; then
        docker cp "$FILENAME" $container_name:"$schemas_path/$FILENAME" >/dev/null 2>&1
        if [ $? -eq 0 ]; then
            echo "${GREEN}Successfully copied $FILENAME to Docker container $container_name.${NC}"
        fi
    else
        echo "${BoldRed}The Layer 2 configuration file has not been downloaded.${NC}"
        echo -e "${BoldGreen}Please download the Layer 2 configuration files by running the download_layer_2_config_bap.sh script located in the ../layer2 folder."
        echo -e "For further information, refer to this URL: https://github.com/beckn/beckn-onix/blob/main/docs/user_guide.md#downloading-layer-2-configuration-for-a-domain.${NC}"
    fi
    rm -f $FILENAME >/dev/null 2>&1
}

# Function to install BAP Protocol Server - creates registry entries only
install_bap_protocol_server() {
    if [[ $1 ]]; then
        registry_url=$1
        bap_subscriber_id=$2
        bap_subscriber_key_id=$3
        bap_subscriber_url=$4
        bash scripts/update_bap_config.sh $registry_url $bap_subscriber_id $bap_subscriber_key_id $bap_subscriber_url $api_key $np_domain
    else
        bash scripts/update_bap_config.sh
    fi
    
    echo "Protocol server BAP registry entries created successfully"
}

# Function to install BPP Protocol Server - creates registry entries only
install_bpp_protocol_server() {
    echo "${GREEN}................Installing Protocol Server for BPP................${NC}"

    if [[ $1 ]]; then
        registry_url=$1
        bpp_subscriber_id=$2
        bpp_subscriber_key_id=$3
        bpp_subscriber_url=$4
        webhook_url=$5
        bash scripts/update_bpp_config.sh $registry_url $bpp_subscriber_id $bpp_subscriber_key_id $bpp_subscriber_url $webhook_url $api_key $np_domain
    else
        bash scripts/update_bpp_config.sh
    fi

    echo "Protocol server BPP registry entries created successfully"
}

mergingNetworks() {
    echo -e "1. Merge Two Different Registries \n2. Merge Multiple Registries into a Super Registry"
    read -p "Enter your choice: " merging_network
    urls=()
    if [ "$merging_network" = "2" ]; then
        while true; do
            read -p "Enter registry URL (or 'N' to stop): " url
            if [[ $url == 'N' ]]; then
                break
            else
                urls+=("$url")
            fi
        done
        read -p "Enter the Super Registry URL: " registry_super_url
    else
        read -p "Enter A registry URL: " registry_a_url
        read -p "Enter B registry URL: " registry_b_url
        urls+=("$registry_a_url")

    fi
    if [[ ${#urls[@]} -gt 0 ]]; then
        echo "Entered registry URLs:"
        all_responses=""
        for url in "${urls[@]}"; do
            response=$(curl -s -H 'ACCEPT: application/json' -H 'CONTENT-TYPE: application/json' "$url"+/subscribers/lookup -d '{}')
            all_responses+="$response"
        done
        for element in $(echo "$all_responses" | jq -c '.[]'); do
            if [ "$merging_network" -eq 1 ]; then
                curl --location "$registry_b_url"+/subscribers/register \
                    --header 'Content-Type: application/json' \
                    --data "$element"
                echo
            else
                curl --location "$registry_super_url"+/subscribers/register \
                    --header 'Content-Type: application/json' \
                    --data "$element"
                echo
            fi
        done
        echo "Merging Multiple Registries into a Super Registry Done ..."
    else
        echo "No registry URLs entered."
    fi

    if [ "$merging_network" = "2" ]; then
        echo "Merging Multiple Registries into a Super Registry"
    else
        echo "Invalid option. Please restart the script and select a valid option."
        exit 1
    fi
}

# Function to install BPP Protocol Server with Sandbox
install_bpp_protocol_server_with_sandbox() {
    echo "${GREEN}................Installing Sandbox................${NC}"
    start_container $bpp_docker_compose_file_sandbox "sandbox-api"
    sleep 5
    echo "Sandbox installation successful"

    echo "${GREEN}................Installing Protocol Server for BPP................${NC}"

    if [[ $1 ]]; then
        registry_url=$1
        bpp_subscriber_id=$2
        bpp_subscriber_key_id=$3
        bpp_subscriber_url=$4
        webhook_url=$5
        bash scripts/update_bpp_config.sh $registry_url $bpp_subscriber_id $bpp_subscriber_key_id $bpp_subscriber_url $webhook_url $api_key $np_domain
    else
        bash scripts/update_bpp_config.sh
    fi

    echo "Protocol server BPP registry entries created successfully"
}

layer2_config() {
    while true; do
        read -p "Paste the URL of the Layer 2 configuration here (or press Enter to skip): " layer2_url
        if [[ -z "$layer2_url" ]]; then
            break #If URL is empty then skip the URL validation
        elif [[ $layer2_url =~ ^(http|https):// ]]; then
            break
        else
            echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
        fi
    done
}

# Validate the user credentials against the Registry
validate_user() {
    # Prompt for username
    read -p "Enter your registry username: " username

    # Prompt for password with '*' masking
    echo -n "Enter your registry password: "
    stty -echo # Disable terminal echo

    password=""
    while IFS= read -r -n1 char; do
        if [[ "$char" == $'\0' ]]; then
            break
        fi
        password+="$char"
        echo -n "*" # Display '*' for each character typed
    done
    stty echo # Re-enable terminal echo
    echo      # Move to a new line after input

    # Replace '/subscribers' with '/login' for validation
    local login_url="${registry_url%/subscribers}/login"

    # Validate credentials using a POST request
    local response
    response=$(curl -s -w "%{http_code}" -X POST "$login_url" \
        -H "Content-Type: application/json" \
        -d '{ "Name" : "'"$username"'", "Password" : "'"$password"'" }')

    # Check if the HTTP response is 200 (success)
    status_code="${response: -3}"
    if [ "$status_code" -eq 200 ]; then
        response_body="${response%???}"
        api_key=$(echo "$response_body" | jq -r '.api_key')
        return 0
    else
        response=$(curl -s -w "%{http_code}" -X POST "$login_url" \
            -H "Content-Type: application/json" \
            -d '{ "User" : { "Name" : "'"$username"'", "Password" : "'"$password"'" }}')

        status_code="${response: -3}"
        if [ "$status_code" -eq 200 ]; then
            response_body="${response%???}"
            api_key=$(echo "$response_body" | jq -r '.api_key')
            return 0
        fi
    fi
    echo "Please check your credentials or register new user on $login_url"
    return 1
}

get_np_domain() {
    if [[ $2 ]]; then
        read -p "Do you want to setup this $1 and $2 for specific domain? {Y/N} " dchoice
    else
        read -p "Do you want to setup this $1 for specific domain? {Y/N} " dchoice
    fi

    if [[ "$dchoice" == "Y" || "$dchoice" == "y" ]]; then
        local login_url="${registry_url%/subscribers}"
        read -p "Enter the domain name for $1 : " np_domain
        domain_present=$(curl -s -H "ApiKey:$api_key" --header 'Content-Type: application/json' $login_url/network_domains/index | jq -r '.[].name' | tr '\n' ' ')
        if echo "$domain_present" | grep -Fqw "$np_domain"; then
            return 0
        else
            echo "${BoldRed}The domain '$np_domain' is NOT present in the network domains.${NC}"
            echo "${BoldGreen}Available network domains: $domain_present ${NC}"
        fi
    else
        np_domain=" " #If user don't want to add specific domain then save empty string
        return 0
    fi
}

# Function to handle the setup process for each platform
completeSetup() {
    platform=$1

    public_address="https://<your public IP address>"

    echo "Proceeding with the setup for $platform..."

    case $platform in
    "Registry")
        while true; do
            read -p "Enter publicly accessible registry URL: " registry_url
            if [[ $registry_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        new_registry_url="${registry_url%/}"
        public_address=$registry_url
        install_package
        install_registry $new_registry_url
        ;;
    "Gateway" | "Beckn Gateway")
        while true; do
            read -p "Enter your registry URL: " registry_url
            if [[ $registry_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        while true; do
            read -p "Enter publicly accessible gateway URL: " gateway_url
            if [[ $gateway_url =~ ^(http|https):// ]]; then
                gateway_url="${gateway_url%/}"
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        public_address=$gateway_url
        install_package
        install_gateway $registry_url $gateway_url
        ;;
    "BAP")
        echo "${GREEN}................Installing Protocol Server for BAP................${NC}"

        read -p "Enter BAP Subscriber ID: " bap_subscriber_id
        while true; do
            read -p "Enter BAP Subscriber URL: " bap_subscriber_url
            if [[ $bap_subscriber_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        while true; do
            read -p "Enter the registry URL (e.g., https://registry.becknprotocol.io/subscribers): " registry_url
            if [[ $registry_url =~ ^(http|https):// ]] && [[ $registry_url == */subscribers ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done
        validate_user
        if [ $? -eq 1 ]; then
            exit
        fi

        get_np_domain $bap_subscriber_id
        if [ $? -eq 1 ]; then
            exit
        fi

        bap_subscriber_key_id="$bap_subscriber_id-key"
        public_address=$bap_subscriber_url

        # layer2_config  # Commented out - ONIX adapter handles schemas differently
        install_package
        install_bap_protocol_server $registry_url $bap_subscriber_id $bap_subscriber_key_id $bap_subscriber_url
        
        # Ask if user wants ONIX adapter
        read -p "${GREEN}Do you want to install ONIX adapter (Y/N): ${NC}" onix_choice
        if [[ "$onix_choice" == "Y" || "$onix_choice" == "y" ]]; then
            install_bap_adapter
        fi
        ;;
    "BPP")
        echo "${GREEN}................Installing Protocol Server for BPP................${NC}"

        read -p "Enter BPP Subscriber ID: " bpp_subscriber_id
        while true; do
            read -p "Enter BPP Subscriber URL: " bpp_subscriber_url
            if [[ $bpp_subscriber_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        while true; do
            read -p "Enter Webhook URL: " webhook_url
            if [[ $webhook_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        while true; do
            read -p "Enter the registry URL (e.g., https://registry.becknprotocol.io/subscribers): " registry_url
            if [[ $registry_url =~ ^(http|https):// ]] && [[ $registry_url == */subscribers ]]; then
                break
            else
                echo "${RED}Please mention /subscribers in your registry URL${NC}"
            fi
        done
        validate_user
        if [ $? -eq 1 ]; then
            exit
        fi

        get_np_domain $bpp_subscriber_id
        if [ $? -eq 1 ]; then
            exit
        fi

        bpp_subscriber_key_id="$bpp_subscriber_id-key"
        public_address=$bpp_subscriber_url

        # layer2_config  # Commented out - ONIX adapter handles schemas differently
        install_package
        install_bpp_protocol_server $registry_url $bpp_subscriber_id $bpp_subscriber_key_id $bpp_subscriber_url $webhook_url
        
        # Ask if user wants ONIX adapter
        read -p "${GREEN}Do you want to install ONIX adapter  (Y/N): ${NC}" onix_choice
        if [[ "$onix_choice" == "Y" || "$onix_choice" == "y" ]]; then
            install_bpp_adapter
        fi
        ;;
    "ALL")
        # Collect all inputs at once for all components

        # Registry input
        while true; do
            read -p "Enter publicly accessible registry URL: " registry_url
            if [[ $registry_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        # Gateway inputs
        while true; do
            read -p "Enter publicly accessible gateway URL: " gateway_url
            if [[ $gateway_url =~ ^(http|https):// ]]; then
                gateway_url="${gateway_url%/}"
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        # BAP inputs
        read -p "Enter BAP Subscriber ID: " bap_subscriber_id
        while true; do
            read -p "Enter BAP Subscriber URL: " bap_subscriber_url
            if [[ $bap_subscriber_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        # BPP inputs
        read -p "Enter BPP Subscriber ID: " bpp_subscriber_id
        while true; do
            read -p "Enter BPP Subscriber URL: " bpp_subscriber_url
            if [[ $bpp_subscriber_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        while true; do
            read -p "Enter Webhook URL: " webhook_url
            if [[ $webhook_url =~ ^(http|https):// ]]; then
                break
            else
                echo "${RED}Invalid URL format. Please enter a valid URL starting with http:// or https://.${NC}"
            fi
        done

        # Install components after gathering all inputs
        install_package

        install_registry $registry_url

        install_gateway $registry_url $gateway_url

        # layer2_config  # Commented out - ONIX adapter handles schemas differently
        #Append /subscribers for registry_url
        new_registry_url="${registry_url%/}/subscribers"
        bap_subscriber_key_id="$bap_subscriber_id-key"
        install_bap_protocol_server $new_registry_url $bap_subscriber_id $bap_subscriber_key_id $bap_subscriber_url

        bpp_subscriber_key_id="$bpp_subscriber_id-key"
        install_bpp_protocol_server $new_registry_url $bpp_subscriber_id $bpp_subscriber_key_id $bpp_subscriber_url $webhook_url
        
        # Ask if user wants ONIX adapter
        read -p "${GREEN}Do you want to install ONIX adapter (Y/N): ${NC}" onix_choice
        if [[ "$onix_choice" == "Y" || "$onix_choice" == "y" ]]; then
            install_adapter "BOTH"
        fi
        ;;
    *)
        echo "Unknown platform: $platform"
        ;;
    esac
}

restart_script() {
    read -p "${GREEN}Do you want to restart the script or exit the script? (r for restart, e for exit): ${NC}" choice
    if [[ $choice == "r" ]]; then
        echo "Restarting the script..."
        exec "$0" # Restart the script by re-executing it
    elif [[ $choice == "e" ]]; then
        echo "Exiting the script..."
        exit 0
    fi
}

# Function to validate user input
validate_input() {
    local input=$1
    local max_option=$2

    # Check if the input is a digit and within the valid range
    if [[ "$input" =~ ^[0-9]+$ ]] && ((input >= 1 && input <= max_option)); then
        return 0 # Valid input
    else
        echo "${RED}Invalid input. Please enter a number between 1 and $max_option.${NC}"
        return 1 # Invalid input
    fi
}

check_docker_permissions() {
    if ! command -v docker &>/dev/null; then
        echo -e "${RED}Error: Docker is not installed on this system.${NC}"
        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            install_package
            if [[ $? -ne 0 ]]; then
                echo -e "${RED}Please install Docker and try again.${NC}"
                echo -e "${RED}Please install Docker and jq manually.${NC}"
                exit 1
            fi
        fi
    fi
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if ! groups "$USER" | grep -q '\bdocker\b'; then
            echo -e "${RED}Error: You do not have permission to run Docker. Please add yourself to the docker group by running the following command:${NC}"
            echo -e "${BoldGreen}sudo usermod -aG docker \$USER"
            echo -e "After running the above command, please log out and log back in to your system, then restart the deployment script.${NC}"
            exit 1
        fi
    fi
}

# Function to update/upgrade a specific service
update_service() {
    service_name=$1
    docker_compose_file=$2
    image_name=$3

    echo "${GREEN}................Updating $service_name................${NC}"

    export COMPOSE_IGNORE_ORPHANS=1
    # Pull the latest image
    docker pull "$image_name"

    # Stop and remove the existing container
    docker compose -f "$docker_compose_file" stop "$service_name"
    docker compose -f "$docker_compose_file" rm -f "$service_name"

    # Start the service with the new image
    docker compose -f "$docker_compose_file" up -d "$service_name"

    echo "$service_name update successful"
}

# Function to validate required modules exist in config file
validate_config_modules() {
    local key_type=$1
    local config_file=$2
    
    local has_bap_modules=$(grep -c "name: bapTxn" "$config_file")
    local has_bpp_modules=$(grep -c "name: bppTxn" "$config_file")
    
    # Only validate that required modules exist, don't enforce strict matching
    if [[ "$key_type" == "BAP" && $has_bap_modules -eq 0 ]]; then
        echo "${RED}Error: BAP deployment selected but no BAP modules found in config file${NC}"
        echo "${BLUE}Config file: $config_file${NC}"
        echo "${YELLOW}Please update docker-compose-adapter.yml to use a config file with BAP modules${NC}"
        return 1
    elif [[ "$key_type" == "BPP" && $has_bpp_modules -eq 0 ]]; then
        echo "${RED}Error: BPP deployment selected but no BPP modules found in config file${NC}"
        echo "${BLUE}Config file: $config_file${NC}"
        echo "${YELLOW}Please update docker-compose-adapter.yml to use a config file with BPP modules${NC}"
        return 1
    elif [[ "$key_type" == "BOTH" && ($has_bap_modules -eq 0 || $has_bpp_modules -eq 0) ]]; then
        echo "${RED}Error: Combined deployment selected but missing modules in config file${NC}"
        echo "${BLUE}Config file: $config_file${NC}"
        echo "${BLUE}BAP modules found: $has_bap_modules, BPP modules found: $has_bpp_modules${NC}"
        echo "${YELLOW}Please update docker-compose-adapter.yml to use a config file with both BAP and BPP modules${NC}"
        return 1
    fi
    
    # Allow BAP deployment with combined config (BAP + BPP modules)
    # Allow BPP deployment with combined config (BAP + BPP modules)
    echo "${GREEN}✓ Config validation passed - $key_type deployment with available modules${NC}"
    return 0
}

# Function to configure ONIX adapter with registry keys
# Usage: configure_onix_registry_keys [BAP|BPP|BOTH] [config_file]
configure_onix_registry_keys() {
    local key_type=${1:-"BOTH"}
    local config_file=$2
    
    echo "${GREEN}Setting up ONIX keys ($key_type) from existing protocol server configs...${NC}"
    
    # Determine config file - only auto-detect if not provided
    if [ -z "$config_file" ]; then
        local docker_config=$(grep -o '/app/config/[^"]*' docker-compose-adapter.yml | head -1)
        if [ -z "$docker_config" ]; then
            echo "${RED}Error: Could not find config file in docker-compose-adapter.yml${NC}"
            return 1
        fi
        config_file="${docker_config/\/app\/config\//../config/}"
        echo "${BLUE}detected config file from docker-compose: $config_file${NC}"
    else
        echo "${BLUE}Using specified config file: $config_file${NC}"
    fi
    
    echo "${BLUE}Updating config file: $config_file${NC}"
    
    local bap_config="protocol-server-data/bap-client.yml"
    local bpp_config="protocol-server-data/bpp-client.yml"
    
    # Extract keys based on type
    local bap_private_key bap_public_key bpp_private_key bpp_public_key
    
    if [[ "$key_type" == "BAP" || "$key_type" == "BOTH" ]]; then
        if [ -f "$bap_config" ]; then
            bap_private_key_full=$(awk '/privateKey:/ {print $2}' "$bap_config" | tr -d '"')
            bap_public_key=$(awk '/publicKey:/ {print $2}' "$bap_config" | tr -d '"')
            bap_private_key=$(echo "$bap_private_key_full" | base64 -d | head -c 32 | base64 -w 0)
            echo "${GREEN}✓ Extracted BAP keys from $bap_config${NC}"
        else
            echo "${RED}Error: BAP config file not found at $bap_config${NC}"
            return 1
        fi
    fi
    
    if [[ "$key_type" == "BPP" || "$key_type" == "BOTH" ]]; then
        if [ -f "$bpp_config" ]; then
            bpp_private_key_full=$(awk '/privateKey:/ {print $2}' "$bpp_config" | tr -d '"')
            bpp_public_key=$(awk '/publicKey:/ {print $2}' "$bpp_config" | tr -d '"')
            bpp_private_key=$(echo "$bpp_private_key_full" | base64 -d | head -c 32 | base64 -w 0)
            echo "${GREEN}✓ Extracted BPP keys from $bpp_config${NC}"
        else
            echo "${YELLOW}⚠ BPP config file not found at $bpp_config - BPP modules will be skipped${NC}"
            # Don't return error, just skip BPP key extraction
        fi
    fi
    
    if [ ! -f "$config_file" ]; then
        echo "${RED}Error: ONIX config file not found at $config_file${NC}"
        return 1
    fi
    
    # Validate required modules exist
    validate_config_modules "$key_type" "$config_file"
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    # Detect indentation pattern from the file
    local steps_indent=$(grep -m1 "steps:" "$config_file" | sed 's/steps:.*//' | wc -c)
    steps_indent=$((steps_indent - 1))  # Remove newline
    local plugin_indent=$((steps_indent + 2))
    local config_indent=$((steps_indent + 4))
    
    # Process the config file with dynamic indentation
    awk -v bap_pub="$bap_public_key" -v bap_priv="$bap_private_key" \
        -v bpp_pub="$bpp_public_key" -v bpp_priv="$bpp_private_key" \
        -v key_type="$key_type" \
        -v steps_spaces="$steps_indent" -v plugin_spaces="$plugin_indent" -v config_spaces="$config_indent" '
    BEGIN { 
        module_type = ""
        in_keymanager = 0
        has_keymanager = 0
        # Create indent strings
        steps_indent = sprintf("%*s", steps_spaces, "")
        plugin_indent = sprintf("%*s", plugin_spaces, "")
        config_indent = sprintf("%*s", config_spaces, "")
        steps_pattern = "^" steps_indent "steps:"
    }
    
    # Track module type and reset keymanager flag
    /- name: bapTxn/ { 
        if ((key_type == "BAP" || key_type == "BOTH") && bap_priv != "") module_type = "bap"
        else module_type = ""
        has_keymanager = 0; print; next 
    }
    /- name: bppTxn/ { 
        if ((key_type == "BPP" || key_type == "BOTH") && bpp_priv != "") module_type = "bpp"
        else module_type = ""
        has_keymanager = 0; print; next 
    }
    /^  - name:/ && !/bapTxn/ && !/bppTxn/ { module_type = ""; has_keymanager = 0; print; next }
    
    # Handle keyManager section
    /keyManager:/ {
        in_keymanager = 1
        has_keymanager = 1
        print
        next
    }
    
    # Force simplekeymanager id
    in_keymanager && /id:/ {
        print config_indent "id: simplekeymanager"
        next
    }
    
    # Replace config section entirely
    in_keymanager && /config:/ {
        print config_indent "config:"
        # Skip existing config lines until next plugin or steps
        while (getline) {
            if (match($0, "^" plugin_indent "[a-zA-Z]") || match($0, steps_pattern)) {
                # Add complete config based on module
                if (module_type == "bap") {
                    print config_indent "  networkParticipant: bap-network"
                    print config_indent "  keyId: bap-network-key"
                    print config_indent "  signingPrivateKey: \"" bap_priv "\""
                    print config_indent "  signingPublicKey: \"" bap_pub "\""
                    print config_indent "  encrPrivateKey: \"" bap_priv "\""
                    print config_indent "  encrPublicKey: \"" bap_pub "\""
                } else if (module_type == "bpp") {
                    print config_indent "  networkParticipant: bpp-network"
                    print config_indent "  keyId: bpp-network-key"
                    print config_indent "  signingPrivateKey: \"" bpp_priv "\""
                    print config_indent "  signingPublicKey: \"" bpp_pub "\""
                    print config_indent "  encrPrivateKey: \"" bpp_priv "\""
                    print config_indent "  encrPublicKey: \"" bpp_pub "\""
                }
                in_keymanager = 0
                print
                next
            }
        }
        next
    }
    
    # Exit keyManager on unindented line
    in_keymanager && match($0, "^" plugin_indent "[a-zA-Z]") && !match($0, "^" config_indent) {
        in_keymanager = 0
    }
    
    # Add keyManager if missing before steps
    module_type && match($0, steps_pattern) && !has_keymanager {
        print plugin_indent "keyManager:"
        print config_indent "id: simplekeymanager"
        print config_indent "config:"
        if (module_type == "bap") {
            print config_indent "  networkParticipant: bap-network"
            print config_indent "  keyId: bap-network-key"
            print config_indent "  signingPrivateKey: \"" bap_priv "\""
            print config_indent "  signingPublicKey: \"" bap_pub "\""
            print config_indent "  encrPrivateKey: \"" bap_priv "\""
            print config_indent "  encrPublicKey: \"" bap_pub "\""
        } else if (module_type == "bpp") {
            print config_indent "  networkParticipant: bpp-network"
            print config_indent "  keyId: bpp-network-key"
            print config_indent "  signingPrivateKey: \"" bpp_priv "\""
            print config_indent "  signingPublicKey: \"" bpp_pub "\""
            print config_indent "  encrPrivateKey: \"" bpp_priv "\""
            print config_indent "  encrPublicKey: \"" bpp_pub "\""
        }
    }
    
    # Print all other lines
    { print }
    ' "$config_file" > "${config_file}.tmp"
    
    if [ $? -eq 0 ] && [ -f "${config_file}.tmp" ]; then
        mv "${config_file}.tmp" "$config_file"
        echo "${GREEN}✓ ONIX keys configured with existing registry keys${NC}"
    else
        echo "${RED}Error: AWK script failed to process config file${NC}"
        rm -f "${config_file}.tmp"
        return 1
    fi
    [[ "$key_type" == "BAP" || "$key_type" == "BOTH" ]] && echo "${GREEN}✓ BAP modules using BAP keys${NC}"
    [[ "$key_type" == "BPP" || "$key_type" == "BOTH" ]] && echo "${GREEN}✓ BPP modules using BPP keys${NC}"
    
    return 0
}

# Function to handle the update/upgrade process
update_network() {
    echo -e "\nWhich component would you like to update?\n1. Registry\n2. Gateway\n3. BAP Protocol Server\n4. BPP Protocol Server\n5. All components"
    read -p "Enter your choice: " update_choice

    validate_input "$update_choice" 5
    if [[ $? -ne 0 ]]; then
        restart_script
    fi

    case $update_choice in
    1)
        update_service "registry" "$registry_docker_compose_file" "fidedocker/registry"
        ;;
    2)
        update_service "gateway" "$gateway_docker_compose_file" "fidedocker/gateway"
        ;;
    3)
        update_service "bap-client" "$bap_docker_compose_file" "fidedocker/protocol-server"
        update_service "bap-network" "$bap_docker_compose_file" "fidedocker/protocol-server"
        ;;
    4)
        update_service "bpp-client" "$bpp_docker_compose_file" "fidedocker/protocol-server"
        update_service "bpp-network" "$bpp_docker_compose_file" "fidedocker/protocol-server"
        ;;
    5)
        update_service "registry" "$registry_docker_compose_file" "fidedocker/registry"
        update_service "gateway" "$gateway_docker_compose_file" "fidedocker/gateway"
        update_service "bap-client" "$bap_docker_compose_file" "fidedocker/protocol-server"
        update_service "bap-network" "$bap_docker_compose_file" "fidedocker/protocol-server"
        update_service "bpp-client" "$bpp_docker_compose_file" "fidedocker/protocol-server"
        update_service "bpp-network" "$bpp_docker_compose_file" "fidedocker/protocol-server"
        ;;
    *)
        echo "Unknown choice"
        ;;
    esac
}

# Function to install ONIX adapter with specified key type
install_adapter() {
    local key_type=${1:-"BOTH"}
    local config_file=$2
    
    # Create schemas directory for validation
    if [ ! -d "schemas" ]; then
        mkdir -p schemas
        echo -e "${GREEN}✓ Created schemas directory${NC}"
    else
        echo -e "${YELLOW}schemas directory already exists${NC}"
    fi

    echo "${GREEN}................Building plugins for ONIX Adapter................${NC}"
    
    # Build plugins the same way as setup.sh
    cd ..
    if [ -f "./install/build-plugins.sh" ]; then
        chmod +x ./install/build-plugins.sh
        ./install/build-plugins.sh
        if [ $? -eq 0 ]; then
            echo "${GREEN}✓ Plugins built successfully${NC}"
        else
            echo "${RED}Error: Plugin build failed${NC}"
            exit 1
        fi
    else
        echo "${RED}Error: install/build-plugins.sh not found${NC}"
        exit 1
    fi
    cd install
    
    echo "${GREEN}................Setting up keys for ONIX Adapter ($key_type)................${NC}"
    configure_onix_registry_keys "$key_type" "$config_file"
    if [ $? -ne 0 ]; then
        echo "${RED}ONIX Adapter installation failed due to configuration errors${NC}"
        exit 1
    fi
    
    echo "${GREEN}................Starting Redis and ONIX Adapter................${NC}"
    start_support_services
    start_container $adapter_docker_compose_file "onix-adapter"
    sleep 10
    echo "ONIX Adapter installation successful"
}

# Helper function for BAP-only ONIX setup
install_bap_adapter() {
    install_adapter "BAP"
}

# Helper function for BPP-only ONIX setup  
install_bpp_adapter() {
    install_adapter "BPP"
}

# Function to install Beckn One Adapter (Option 3)
install_beckn_one_adapter() {
    echo "${GREEN}................Setting up network with Beckn One................${NC}"
    
    # Create schemas directory if not exists
    if [ ! -d "schemas" ]; then
        mkdir -p schemas
        echo -e "${GREEN}✓ Created schemas directory${NC}"
    fi

    echo "${GREEN}................Building plugins for ONIX Adapter................${NC}"
    
    # Build plugins
    cd ..
    if [ -f "./install/build-plugins.sh" ]; then
        chmod +x ./install/build-plugins.sh
        ./install/build-plugins.sh
        if [ $? -eq 0 ]; then
            echo "${GREEN}✓ Plugins built successfully${NC}"
        else
            echo "${RED}Error: Plugin build failed${NC}"
            exit 1
        fi
    else
        echo "${RED}Error: install/build-plugins.sh not found${NC}"
        exit 1
    fi
    cd install
    
    echo "${GREEN}................Starting Redis and Beckn One Adapters................${NC}"
    
    # Start Redis first
    start_support_services
    
    # Start Beckn One adapters (onix-bap, onix-bpp, sandbox-bap, sandbox-bpp)
    start_container $adapter_beckn_one_docker_compose_file "onix-bap"
    start_container $adapter_beckn_one_docker_compose_file "onix-bpp"
    start_container $adapter_beckn_one_docker_compose_file "sandbox-bap"
    start_container $adapter_beckn_one_docker_compose_file "sandbox-bpp"
    
    sleep 10
    
    echo "${GREEN}✓ Beckn One network setup complete${NC}"
    echo ""
    echo "${BLUE}Services running:${NC}"
    echo "  - ONIX BAP Adapter: http://localhost:8081"
    echo "  - ONIX BPP Adapter: http://localhost:8082"
    echo "  - Sandbox BAP: http://localhost:3001"
    echo "  - Sandbox BPP: http://localhost:3002"
    echo "  - Redis: localhost:6379"
}
# MAIN SCRIPT STARTS HERE

echo "Welcome to Beckn-ONIX!"
if [ -f ./onix_ascii_art.txt ]; then
    cat ./onix_ascii_art.txt
else
    echo "[Display Beckn-ONIX ASCII Art]"
fi

echo "Checking prerequisites of Beckn-ONIX deployment"
check_docker_permissions

echo "Beckn-ONIX is a platform that helps you quickly launch and configure beckn-enabled networks."
echo -e "\nWhat would you like to do?\n1. Join an existing network\n2. Create new production network\n3. Set up a network on local machine with Beckn One (DeDI-Registry, Catalog Discovery)\n4. Set up a network on local machine with local registry and gateway (without Beckn One)\n5. Merge multiple networks\n6. Configure Existing Network\n7. Update/Upgrade Application\n(Press Ctrl+C to exit)"
read -p "Enter your choice: " choice

validate_input "$choice" 7
if [[ $? -ne 0 ]]; then
    restart_script # Restart the script if input is invalid
fi

if [[ $choice -eq 3 ]]; then
    echo "${GREEN}Setting up network with Beckn One${NC}"
    echo "${BLUE}This setup uses Beckn One (DeDI-Registry) for registry and catalog discovery${NC}"
    echo ""
    install_package
    install_beckn_one_adapter
elif [[ $choice -eq 4 ]]; then
    echo "Installing network with local registry and gateway (without Beckn One)"
    install_package
    install_registry
    install_gateway
    install_bap_protocol_server
    install_bpp_protocol_server_with_sandbox
    install_adapter "BOTH"
elif [[ $choice -eq 5 ]]; then
    echo "Determining the platforms available based on the initial choice"
    mergingNetworks
elif [[ $choice -eq 6 ]]; then
    echo "${BoldGreen}Currently this feature is not available in this distribution of Beckn ONIX${NC}"
    restart_script
elif [[ $choice -eq 7 ]]; then
    update_network
else
    # Determine the platforms available based on the initial choice
    platforms=("Gateway" "BAP" "BPP" "ALL")
    [ "$choice" -eq 2 ] && platforms=("Registry" "${platforms[@]}") # Add Registry for new network setups

    echo "Great choice! Get ready."
    echo -e "\nWhich platform would you like to set up?"
    for i in "${!platforms[@]}"; do
        echo "$((i + 1)). ${platforms[$i]}"
    done

    read -p "Enter your choice: " platform_choice
    validate_input "$platform_choice" "${#platforms[@]}"
    if [[ $? -ne 0 ]]; then
        restart_script # Restart the script if input is invalid
    fi

    selected_platform="${platforms[$((platform_choice - 1))]}"

    if [[ -n $selected_platform ]]; then
        completeSetup "$selected_platform"
    else
        restart_script
    fi
fi

echo "Process complete. Thank you for using Beckn-ONIX!"