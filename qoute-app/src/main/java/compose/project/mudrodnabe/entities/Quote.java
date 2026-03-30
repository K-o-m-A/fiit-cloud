package compose.project.mudrodnabe.entities;

import lombok.Data;
import org.springframework.data.annotation.Id;
import org.springframework.data.mongodb.core.mapping.Document;

@Data
@Document(collection = "quote")
public class Quote {
    @Id
    private String id;
    private String quote;
}
