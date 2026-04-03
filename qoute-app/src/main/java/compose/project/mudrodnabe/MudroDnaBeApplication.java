package compose.project.mudrodnabe;

import io.mongock.runner.springboot.EnableMongock;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
@EnableMongock
public class MudroDnaBeApplication {
    public static void main(String[] args) {
        SpringApplication.run(MudroDnaBeApplication.class, args);
    }

}
